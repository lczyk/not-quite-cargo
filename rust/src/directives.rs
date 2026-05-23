use std::collections::HashMap;

use crate::logger::Logger;

#[derive(Clone, Debug, Default)]
pub struct CustomBuildDirectives {
    pub rustc_flags: Vec<String>,
    pub env_vars: HashMap<String, String>,
}

pub fn parse_build_script_output(output: &str, logger: &Logger) -> CustomBuildDirectives {
    let mut d = CustomBuildDirectives::default();

    for raw in output.lines() {
        let line = raw.trim();
        if !line.starts_with("cargo:") {
            continue;
        }
        let line = &line["cargo:".len()..];

        let (key, value) = match line.split_once('=') {
            Some((k, v)) => (k, v),
            None => {
                logger.warn(&format!("malformed build script line (no '='): {raw}"));
                continue;
            }
        };

        if matches!(key, "rerun-if-changed" | "rerun-if-env-changed") {
            continue;
        }

        if value.is_empty() {
            logger.warn(&format!(
                "empty value for build script directive {key}: {raw}"
            ));
            continue;
        }

        match key {
            "rustc-cfg" => {
                d.rustc_flags.push("--cfg".to_string());
                d.rustc_flags.push(value.to_string());
            }
            "rustc-check-cfg" => {
                d.rustc_flags.push("--check-cfg".to_string());
                d.rustc_flags.push(value.to_string());
            }
            "rustc-link-lib" => {
                d.rustc_flags.push("-l".to_string());
                d.rustc_flags.push(value.to_string());
            }
            "rustc-link-arg" => {
                d.rustc_flags.push("-C".to_string());
                d.rustc_flags.push(format!("link-arg={value}"));
            }
            "rustc-link-search" => {
                d.rustc_flags.push("-L".to_string());
                if let Some((_kind, path)) = value.split_once('=') {
                    d.rustc_flags.push(path.to_string());
                } else {
                    d.rustc_flags.push(value.to_string());
                }
            }
            "rustc-env" => {
                if let Some((k, v)) = value.split_once('=') {
                    if !k.is_empty() {
                        d.env_vars.insert(k.to_string(), v.to_string());
                    } else {
                        logger.warn(&format!("malformed rustc-env directive: {raw}"));
                    }
                } else {
                    logger.warn(&format!("malformed rustc-env directive: {raw}"));
                }
            }
            _ => {
                logger.warn(&format!("unknown build script directive: {raw}"));
            }
        }
    }

    d
}

#[cfg(test)]
mod tests {
    use super::*;

    fn logger() -> Logger {
        Logger::new()
    }

    #[test]
    fn parses_rustc_cfg() {
        let d = parse_build_script_output("cargo:rustc-cfg=foo", &logger());
        assert_eq!(d.rustc_flags, vec!["--cfg", "foo"]);
    }

    #[test]
    fn parses_rustc_link_search_with_kind() {
        let d = parse_build_script_output("cargo:rustc-link-search=native=/path/to/lib", &logger());
        assert_eq!(d.rustc_flags, vec!["-L", "/path/to/lib"]);
    }

    #[test]
    fn parses_rustc_link_search_without_kind() {
        let d = parse_build_script_output("cargo:rustc-link-search=/path/to/lib", &logger());
        assert_eq!(d.rustc_flags, vec!["-L", "/path/to/lib"]);
    }

    #[test]
    fn parses_rustc_env() {
        let d = parse_build_script_output("cargo:rustc-env=OUT_DIR=/path/to/out", &logger());
        assert_eq!(d.env_vars.get("OUT_DIR").unwrap(), "/path/to/out");
    }

    #[test]
    fn ignores_rerun_directives() {
        let d = parse_build_script_output(
            "cargo:rerun-if-changed=build.rs\ncargo:rerun-if-env-changed=CC",
            &logger(),
        );
        assert!(d.rustc_flags.is_empty());
        assert!(d.env_vars.is_empty());
    }

    #[test]
    fn skips_non_cargo_lines() {
        let d = parse_build_script_output("warning: some warning\ncargo:rustc-cfg=bar", &logger());
        assert_eq!(d.rustc_flags, vec!["--cfg", "bar"]);
    }

    #[test]
    fn warns_on_unknown_directive() {
        let d = parse_build_script_output("cargo:unknown=foo", &logger());
        assert!(d.rustc_flags.is_empty());
    }

    #[test]
    fn parses_rustc_link_lib() {
        let d = parse_build_script_output("cargo:rustc-link-lib=static=foo", &logger());
        assert_eq!(d.rustc_flags, vec!["-l", "static=foo"]);
    }

    #[test]
    fn parses_rustc_link_arg() {
        let d = parse_build_script_output("cargo:rustc-link-arg=-Wl,-rpath,/lib", &logger());
        assert_eq!(d.rustc_flags, vec!["-C", "link-arg=-Wl,-rpath,/lib"]);
    }

    #[test]
    fn parses_rustc_check_cfg() {
        let d = parse_build_script_output("cargo:rustc-check-cfg=values(feature)", &logger());
        assert_eq!(d.rustc_flags, vec!["--check-cfg", "values(feature)"]);
    }

    #[test]
    fn multiple_directives() {
        let output =
            "cargo:rustc-cfg=foo\ncargo:rustc-env=KEY=val\ncargo:rustc-link-search=native=/lib";
        let d = parse_build_script_output(output, &logger());
        assert_eq!(d.rustc_flags.len(), 4); // --cfg, foo, -L, /lib
        assert_eq!(d.env_vars.get("KEY").unwrap(), "val");
    }
}
