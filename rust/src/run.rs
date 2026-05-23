use std::collections::HashMap;
use std::path::Path;
use std::process::Command;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::thread;

use anyhow::Context;

use crate::config::{self, Config};
use crate::directives;
use crate::plan::{self, Invocation};
use crate::topo;

pub fn run(path: &Path, cfg: &Config) -> anyhow::Result<()> {
    let logger = &cfg.logger;

    // Verify rustc works.
    match Command::new(&cfg.rustc_path).arg("-vV").output() {
        Ok(out) => {
            let first_line = String::from_utf8_lossy(&out.stdout)
                .lines()
                .next()
                .unwrap_or("")
                .to_string();
            logger.info(&format!("rustc version: {first_line}"));
        }
        Err(e) => {
            return Err(e)
                .with_context(|| format!("get rustc version from {}", cfg.rustc_path.display()));
        }
    }

    let plan_value = plan::load_plan_json(path)?;

    let invs_raw = match &plan_value {
        tinyjson::JsonValue::Object(entries) => entries
            .iter()
            .find(|(k, _)| k.as_str() == "invocations")
            .and_then(|(_, v)| match v {
                tinyjson::JsonValue::Array(arr) => Some(arr.clone()),
                _ => None,
            }),
        _ => None,
    }
    .unwrap_or_default();

    let replacements = cfg.replacements();
    let replaced: Vec<tinyjson::JsonValue> = invs_raw
        .iter()
        .map(|inv| crate::deepreplace::deep_replace(inv, &replacements))
        .collect();

    let invs = plan::decode_invocations(&replaced)?;

    // Run topo for cycle detection, ignore the order.
    let _ordered = topo::resolve_invocation_order(&invs)?;

    // Pre-create output directories.
    for inv in &invs {
        for out in &inv.outputs {
            if let Some(parent) = Path::new(out).parent() {
                std::fs::create_dir_all(parent)
                    .with_context(|| format!("mkdir for output {out}: {}", parent.display()))?;
            }
        }
    }

    if cfg.jobs <= 1 {
        run_serial(&invs, cfg)
    } else {
        run_parallel(&invs, cfg)
    }
}

fn run_serial(invs: &[Invocation], cfg: &Config) -> anyhow::Result<()> {
    let logger = &cfg.logger;
    let mut directives_map: HashMap<String, directives::CustomBuildDirectives> = HashMap::new();
    let ordered = topo::resolve_invocation_order(invs)?;

    for (i, inv) in ordered.iter().enumerate() {
        let snapshot = directives_map.get(&inv.package_name).cloned();
        let res = execute_invocation(inv, cfg, snapshot.as_ref(), Some((i + 1, ordered.len())))?;
        if let Some(d) = res {
            directives_map.insert(inv.package_name.clone(), d);
        }
    }

    logger.info("build plan execution complete");
    Ok(())
}

fn run_parallel(invs: &[Invocation], cfg: &Config) -> anyhow::Result<()> {
    let logger = &cfg.logger;
    let n = invs.len();

    // Map Invocation.number -> index in invs.
    let mut id_to_idx: HashMap<i32, usize> = HashMap::with_capacity(n);
    for (idx, inv) in invs.iter().enumerate() {
        id_to_idx.insert(inv.number, idx);
    }

    // Build pending_deps + dependents.
    let mut pending_deps: Vec<usize> = Vec::with_capacity(n);
    let mut dependents: Vec<Vec<usize>> = vec![Vec::new(); n];
    for (idx, inv) in invs.iter().enumerate() {
        let mut count = 0usize;
        for dep in &inv.deps {
            if let Some(&dep_idx) = id_to_idx.get(dep) {
                dependents[dep_idx].push(idx);
                count += 1;
            }
        }
        pending_deps.push(count);
    }

    let state = Arc::new(SharedState {
        invs: invs.to_vec(),
        pending_deps: pending_deps.into_iter().map(AtomicUsize::new).collect(),
        dependents,
        ready: Mutex::new(Vec::new()),
        ready_cv: Condvar::new(),
        directives_map: Mutex::new(HashMap::new()),
        in_flight: AtomicUsize::new(0),
        completed: AtomicUsize::new(0),
        total: n,
        cancel: AtomicBool::new(false),
        error: Mutex::new(None),
    });

    // Seed ready queue with invs that have zero deps.
    {
        let mut q = state.ready.lock().unwrap();
        for (idx, count) in state.pending_deps.iter().enumerate() {
            if count.load(Ordering::SeqCst) == 0 {
                q.push(idx);
            }
        }
        state.ready_cv.notify_all();
    }

    let mut handles = Vec::with_capacity(cfg.jobs);
    for _ in 0..cfg.jobs {
        let state = Arc::clone(&state);
        let cfg = cfg.clone();
        handles.push(thread::spawn(move || worker(state, cfg)));
    }

    for h in handles {
        let _ = h.join();
    }

    if let Some(err) = state.error.lock().unwrap().take() {
        return Err(err);
    }

    if state.completed.load(Ordering::SeqCst) != n {
        anyhow::bail!(
            "parallel run terminated with {} of {} invocations completed",
            state.completed.load(Ordering::SeqCst),
            n
        );
    }

    logger.info("build plan execution complete");
    Ok(())
}

struct SharedState {
    invs: Vec<Invocation>,
    pending_deps: Vec<AtomicUsize>,
    dependents: Vec<Vec<usize>>,
    ready: Mutex<Vec<usize>>,
    ready_cv: Condvar,
    directives_map: Mutex<HashMap<String, directives::CustomBuildDirectives>>,
    in_flight: AtomicUsize,
    completed: AtomicUsize,
    total: usize,
    cancel: AtomicBool,
    error: Mutex<Option<anyhow::Error>>,
}

fn worker(state: Arc<SharedState>, cfg: Config) {
    loop {
        let idx = {
            let mut q = state.ready.lock().unwrap();
            loop {
                if state.cancel.load(Ordering::SeqCst) {
                    return;
                }
                if let Some(idx) = q.pop() {
                    break idx;
                }
                // No work available. If nothing in flight and queue empty, done.
                if state.in_flight.load(Ordering::SeqCst) == 0
                    && state.completed.load(Ordering::SeqCst) == state.total
                {
                    state.ready_cv.notify_all();
                    return;
                }
                q = state.ready_cv.wait(q).unwrap();
            }
        };

        state.in_flight.fetch_add(1, Ordering::SeqCst);

        let inv = &state.invs[idx];
        let snapshot = state
            .directives_map
            .lock()
            .unwrap()
            .get(&inv.package_name)
            .cloned();
        let position = (
            state.completed.load(Ordering::SeqCst) + 1,
            state.total,
        );

        let result = execute_invocation(inv, &cfg, snapshot.as_ref(), Some(position));

        match result {
            Ok(maybe_dir) => {
                if let Some(d) = maybe_dir {
                    state
                        .directives_map
                        .lock()
                        .unwrap()
                        .insert(inv.package_name.clone(), d);
                }
                state.completed.fetch_add(1, Ordering::SeqCst);
                // Notify dependents.
                let mut q = state.ready.lock().unwrap();
                for &dep_idx in &state.dependents[idx] {
                    let prev = state.pending_deps[dep_idx].fetch_sub(1, Ordering::SeqCst);
                    if prev == 1 {
                        q.push(dep_idx);
                    }
                }
                state.in_flight.fetch_sub(1, Ordering::SeqCst);
                state.ready_cv.notify_all();
            }
            Err(e) => {
                state.cancel.store(true, Ordering::SeqCst);
                let mut slot = state.error.lock().unwrap();
                if slot.is_none() {
                    *slot = Some(e);
                }
                state.in_flight.fetch_sub(1, Ordering::SeqCst);
                state.ready_cv.notify_all();
                return;
            }
        }
    }
}

/// Execute a single invocation. Returns Some(directives) iff the
/// invocation was a build-script run that produced cargo directives.
fn execute_invocation(
    inv: &Invocation,
    cfg: &Config,
    snapshot: Option<&directives::CustomBuildDirectives>,
    position: Option<(usize, usize)>,
) -> anyhow::Result<Option<directives::CustomBuildDirectives>> {
    let logger = &cfg.logger;

    let mut args = inv.args.clone();
    let program = if inv.program.is_empty() {
        cfg.rustc_path.display().to_string()
    } else {
        inv.program.clone()
    };

    // Apply directives from previously-run build scripts for same package.
    let mut inv_env = inv.env.clone();
    if let Some(d) = snapshot {
        args.extend(d.rustc_flags.clone());
        for (k, v) in &d.env_vars {
            inv_env.insert(k.clone(), v.clone());
        }
    }

    // Pre-create OUT_DIR if set.
    if let Some(out_dir) = inv_env.get("OUT_DIR") {
        std::fs::create_dir_all(out_dir).with_context(|| format!("mkdir OUT_DIR {out_dir}"))?;
    }

    let prog_path = std::path::Path::new(&program);

    let resolved = if prog_path.is_absolute() || prog_path.components().count() > 1 {
        program.clone()
    } else {
        config::look_path(&program)
            .map(|p| p.display().to_string())
            .unwrap_or(program.clone())
    };

    if let Some((cur, total)) = position {
        logger.info(&format!(
            "({}/{}) running '{}' for package '{}' v{}",
            cur,
            total,
            if inv.program.is_empty() {
                "rustc"
            } else {
                &inv.program
            },
            inv.package_name,
            inv.package_version
        ));
    }

    let args_display = truncate(&args.join(" "), 100);
    logger.info(&format!("invoking: {resolved} {args_display}"));

    let mut cmd = Command::new(&resolved);
    cmd.args(&args);
    cmd.current_dir(&inv.cwd);
    cmd.env_clear();
    for (k, v) in build_env(cfg, &inv_env) {
        cmd.env(k, v);
    }

    let output = cmd.output().with_context(|| {
        format!(
            "invocation {} ({}) failed to start",
            inv.number, inv.package_name
        )
    })?;

    if !output.status.success() {
        logger.warn(&format!("command failed: {resolved} {}", args.join(" ")));
        logger.warn(&format!(
            "stdout:\n{}",
            String::from_utf8_lossy(&output.stdout)
        ));
        logger.warn(&format!(
            "stderr:\n{}",
            String::from_utf8_lossy(&output.stderr)
        ));
        anyhow::bail!(
            "invocation {} ({}) failed with status {}",
            inv.number,
            inv.package_name,
            output.status
        );
    }

    // Create symlinks.
    for (link, target) in &inv.links {
        if Path::new(link).symlink_metadata().is_ok() {
            std::fs::remove_file(link)
                .with_context(|| format!("remove stale symlink {link}"))?;
        }
        symlink(target, link).with_context(|| format!("create symlink {link} -> {target}"))?;
        logger.info(&format!("symlink: {link} -> {target}"));
    }

    if inv.compile_mode == "run-custom-build" {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let d = directives::parse_build_script_output(&stdout, logger);
        return Ok(Some(d));
    }

    Ok(None)
}

fn build_env(cfg: &Config, inv_env: &HashMap<String, String>) -> HashMap<String, String> {
    let mut merged = HashMap::new();

    for (k, v) in std::env::vars() {
        merged.insert(k, v);
    }

    merged.insert("RUSTC".to_string(), cfg.rustc_path.display().to_string());
    merged.insert(
        "CARGO_HOME".to_string(),
        cfg.cargo_home.display().to_string(),
    );
    merged.insert(
        "PROJECT_ROOT".to_string(),
        cfg.project_root.display().to_string(),
    );

    for (k, v) in inv_env {
        merged.insert(k.clone(), v.clone());
    }

    merged
}

fn truncate(s: &str, n: usize) -> String {
    if s.len() <= n {
        s.to_string()
    } else {
        format!("{}...", &s[..n])
    }
}

#[cfg(unix)]
fn symlink(target: &str, link: &str) -> std::io::Result<()> {
    std::os::unix::fs::symlink(target, link)
}

#[cfg(not(unix))]
fn symlink(_target: &str, _link: &str) -> std::io::Result<()> {
    Err(std::io::Error::new(
        std::io::ErrorKind::Unsupported,
        "symlinks not supported on this platform",
    ))
}
