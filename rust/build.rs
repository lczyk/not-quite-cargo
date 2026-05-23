use std::process::Command;

fn cmd_output(program: &str, args: &[&str], dir: Option<&str>) -> Option<String> {
    let mut c = Command::new(program);
    c.args(args);
    if let Some(d) = dir {
        c.current_dir(d);
    }
    let out = c.output().ok()?;
    if !out.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

fn main() {
    let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap_or_else(|_| ".".into());

    let sha = cmd_output("git", &["rev-parse", "HEAD"], Some(&manifest_dir))
        .filter(|s| !s.is_empty())
        .unwrap_or_default();

    let info = match cmd_output(
        "git",
        &["status", "--porcelain", "--", "."],
        Some(&manifest_dir),
    ) {
        Some(s) if !s.is_empty() => "dirty".to_string(),
        _ => String::new(),
    };

    let date = cmd_output("date", &["-u", "+%Y-%m-%dT%H:%M:%SZ"], None)
        .filter(|s| !s.is_empty())
        .unwrap_or_default();

    println!("cargo:rustc-env=VERSION_COMMIT_SHA={sha}");
    println!("cargo:rustc-env=VERSION_BUILD_DATE={date}");
    println!("cargo:rustc-env=VERSION_BUILD_INFO={info}");

    println!("cargo:rerun-if-changed=build.rs");
    println!("cargo:rerun-if-changed=.git/HEAD");
    println!("cargo:rerun-if-changed=.git/index");
}
