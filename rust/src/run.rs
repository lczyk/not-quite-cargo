use std::collections::HashMap;
use std::path::Path;
use std::process::Command;

use anyhow::Context;

use crate::config::{self, Config};
use crate::directives;
use crate::plan;
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
    let ordered = topo::resolve_invocation_order(&invs)?;

    // Pre-create output directories.
    for inv in &ordered {
        for out in &inv.outputs {
            if let Some(parent) = Path::new(out).parent() {
                std::fs::create_dir_all(parent)
                    .with_context(|| format!("mkdir for output {out}: {}", parent.display()))?;
            }
        }
    }

    let mut directives_map: HashMap<String, directives::CustomBuildDirectives> = HashMap::new();

    for (i, inv) in ordered.iter().enumerate() {
        let mut args = inv.args.clone();
        let program = if inv.program.is_empty() {
            cfg.rustc_path.display().to_string()
        } else {
            inv.program.clone()
        };

        // Apply directives from previously-run build scripts for same package.
        let mut inv_env = inv.env.clone();
        if let Some(d) = directives_map.get(&inv.package_name) {
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

        // Resolve the actual binary to execute: if it's just a name, look it up on PATH.
        let resolved = if prog_path.is_absolute() || prog_path.components().count() > 1 {
            program.clone()
        } else {
            // Look up on PATH.
            config::look_path(&program)
                .map(|p| p.display().to_string())
                .unwrap_or(program.clone())
        };

        logger.info(&format!(
            "({}/{}) running '{}' for package '{}' v{}",
            i + 1,
            ordered.len(),
            if inv.program.is_empty() {
                "rustc"
            } else {
                &inv.program
            },
            inv.package_name,
            inv.package_version
        ));

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

        // Create symlinks. symlink_metadata is used instead of exists()
        // because exists() follows symlinks: a dangling symlink would
        // return false, then symlink() hits EEXIST.
        for (link, target) in &inv.links {
            if Path::new(link).symlink_metadata().is_ok() {
                std::fs::remove_file(link)
                    .with_context(|| format!("remove stale symlink {link}"))?;
            }
            symlink(target, link).with_context(|| format!("create symlink {link} -> {target}"))?;
            logger.info(&format!("symlink: {link} -> {target}"));
        }

        // Parse build script output if this was a custom build step.
        if inv.compile_mode == "run-custom-build" {
            let stdout = String::from_utf8_lossy(&output.stdout);
            let d = directives::parse_build_script_output(&stdout, logger);
            directives_map.insert(inv.package_name.clone(), d);
        }
    }

    logger.info("build plan execution complete");
    Ok(())
}

fn build_env(cfg: &Config, inv_env: &HashMap<String, String>) -> HashMap<String, String> {
    let mut merged = HashMap::new();

    // Start with current process env.
    for (k, v) in std::env::vars() {
        merged.insert(k, v);
    }

    // Override with cfg values.
    merged.insert("RUSTC".to_string(), cfg.rustc_path.display().to_string());
    merged.insert(
        "CARGO_HOME".to_string(),
        cfg.cargo_home.display().to_string(),
    );
    merged.insert(
        "PROJECT_ROOT".to_string(),
        cfg.project_root.display().to_string(),
    );

    // Invocation env overrides everything.
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
