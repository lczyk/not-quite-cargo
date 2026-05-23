use std::collections::HashMap;
use std::path::PathBuf;
use std::process::Command;

use crate::logger::Logger;

#[derive(Clone)]
pub struct Config {
    pub project_root: PathBuf,
    pub cargo_home: PathBuf,
    pub rustc_path: PathBuf,
    pub logger: Logger,
}

impl Config {
    pub fn replacements(&self) -> HashMap<String, String> {
        let mut m = HashMap::new();
        m.insert(
            "{{PROJECT_ROOT}}".to_string(),
            self.project_root.display().to_string(),
        );
        m.insert(
            "{{CARGO_HOME}}".to_string(),
            self.cargo_home.display().to_string(),
        );
        m.insert(
            "{{RUSTC}}".to_string(),
            self.rustc_path.display().to_string(),
        );
        m
    }
}

pub fn new_config(logger: Logger) -> anyhow::Result<Config> {
    let project_root = std::env::var("PROJECT_ROOT")
        .ok()
        .filter(|s| !s.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| std::env::current_dir().expect("cwd"));

    let cargo_home = std::env::var("CARGO_HOME")
        .ok()
        .filter(|s| !s.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| {
            let mut home = home_dir();
            home.push(".cargo");
            home
        });

    let rustc_path = find_rustc(&logger)?;

    Ok(Config {
        project_root,
        cargo_home,
        rustc_path,
        logger,
    })
}

fn home_dir() -> PathBuf {
    std::env::var("HOME")
        .ok()
        .filter(|s| !s.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/"))
}

pub fn look_path(name: &str) -> Option<PathBuf> {
    std::env::var_os("PATH").and_then(|path| {
        std::env::split_paths(&path).find_map(|dir| {
            let full = dir.join(name);
            full.exists().then_some(full)
        })
    })
}

fn find_rustc(logger: &Logger) -> anyhow::Result<PathBuf> {
    if let Ok(path) = std::env::var("RUSTC") {
        if !path.is_empty() {
            logger.info(&format!("found rustc at {path} via RUSTC env"));
            return Ok(PathBuf::from(path));
        }
    }

    if let Some(rustup) = look_path("rustup") {
        let output = Command::new(&rustup)
            .args(["which", "rustc"])
            .current_dir("/")
            .output();
        if let Ok(out) = output {
            if out.status.success() {
                let path = String::from_utf8_lossy(&out.stdout).trim().to_string();
                if !path.is_empty() {
                    logger.info(&format!("found rustc at {path} via rustup"));
                    return Ok(PathBuf::from(path));
                }
            }
        }
    }

    if let Some(path) = look_path("rustc") {
        logger.info(&format!("found rustc at {} via PATH", path.display()));
        return Ok(path);
    }

    anyhow::bail!("could not locate rustc (set RUSTC, install rustup, or put rustc on PATH)")
}
