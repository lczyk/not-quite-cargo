pub fn format_version(
    version: &str,
    commit_sha: &str,
    build_date: &str,
    build_info: &str,
) -> String {
    let mut result = String::from(version);

    if !commit_sha.is_empty() {
        let n = commit_sha.len().min(7);
        result.push_str(" @ ");
        result.push_str(&commit_sha[..n]);
    }

    match (!build_date.is_empty(), !build_info.is_empty()) {
        (true, true) => {
            result.push_str(" (");
            result.push_str(build_date);
            result.push_str(", ");
            result.push_str(build_info);
            result.push(')');
        }
        (true, false) => {
            result.push_str(" (");
            result.push_str(build_date);
            result.push(')');
        }
        (false, true) => {
            result.push_str(" (");
            result.push_str(build_info);
            result.push(')');
        }
        (false, false) => {}
    }

    result
}

#[macro_export]
macro_rules! version {
    () => {
        $crate::version::format_version(
            env!("CARGO_PKG_VERSION"),
            option_env!("VERSION_COMMIT_SHA").unwrap_or(""),
            option_env!("VERSION_BUILD_DATE").unwrap_or(""),
            option_env!("VERSION_BUILD_INFO").unwrap_or(""),
        )
    };
}
