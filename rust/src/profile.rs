use tinyjson::JsonValue;

#[derive(Debug, Clone, Copy)]
pub struct ProfileSpec {
    pub name: &'static str,
    pub opt_level: &'static str,
    pub debuginfo: &'static str,
    pub debug_assertions: &'static str,
    pub overflow_checks: &'static str,
    pub debug_env: &'static str,
}

pub const RELEASE: ProfileSpec = ProfileSpec {
    name: "release",
    opt_level: "3",
    debuginfo: "0",
    debug_assertions: "false",
    overflow_checks: "false",
    debug_env: "false",
};

pub const DEBUG: ProfileSpec = ProfileSpec {
    name: "debug",
    opt_level: "0",
    debuginfo: "2",
    debug_assertions: "true",
    overflow_checks: "true",
    debug_env: "true",
};

pub fn parse_profile(s: &str) -> Option<ProfileSpec> {
    match s {
        "release" => Some(RELEASE),
        "debug" => Some(DEBUG),
        _ => None,
    }
}

/// Override (or add) `-C debuginfo=N` on rustc invocations.
pub fn rewrite_debuginfo(plan: &mut JsonValue, level: &str) {
    if let JsonValue::Object(entries) = plan {
        if let Some((_, JsonValue::Array(invs))) = entries
            .iter_mut()
            .find(|(k, _)| k.as_str() == "invocations")
        {
            for inv in invs.iter_mut() {
                if !is_rustc(inv) {
                    continue;
                }
                if let JsonValue::Object(e) = inv {
                    if let Some((_, JsonValue::Array(args))) =
                        e.iter_mut().find(|(k, _)| k.as_str() == "args")
                    {
                        set_codegen_arg(args, "debuginfo", level);
                    }
                }
            }
        }
    }
}

fn is_rustc(inv: &JsonValue) -> bool {
    let entries = match inv {
        JsonValue::Object(e) => e,
        _ => return false,
    };
    let prog = entries.iter().find(|(k, _)| k.as_str() == "program");
    matches!(prog, Some((_, JsonValue::String(s))) if s == "{{RUSTC}}" || s == "rustc")
}

fn set_codegen_arg(args: &mut Vec<JsonValue>, key: &str, value: &str) {
    let mut i = 0;
    let mut replaced = false;
    while i < args.len() {
        if let JsonValue::String(s) = &args[i] {
            // Two-arg form: ["-C", "key=val"]
            if s == "-C" && i + 1 < args.len() {
                if let JsonValue::String(next) = &args[i + 1] {
                    if let Some((k, _)) = next.split_once('=') {
                        if k == key {
                            args[i + 1] = JsonValue::String(format!("{key}={value}"));
                            replaced = true;
                        }
                    }
                }
                i += 2;
                continue;
            }
            // One-arg form: "-C key=val"
            if let Some(body) = s.strip_prefix("-C ") {
                if let Some((k, _)) = body.split_once('=') {
                    if k == key {
                        args[i] = JsonValue::String(format!("-C {key}={value}"));
                        replaced = true;
                    }
                }
            }
        }
        i += 1;
    }
    if !replaced {
        args.push(JsonValue::String("-C".to_string()));
        args.push(JsonValue::String(format!("{key}={value}")));
    }
}

pub fn rewrite_profile(plan: &mut JsonValue, target: &ProfileSpec) {
    let source = detect_source(plan).unwrap_or(target.name);

    if let JsonValue::Object(entries) = plan {
        if let Some((_, JsonValue::Array(invs))) = entries
            .iter_mut()
            .find(|(k, _)| k.as_str() == "invocations")
        {
            for inv in invs.iter_mut() {
                rewrite_invocation(inv, source, target);
            }
        }
        if let Some((_, JsonValue::Array(inputs))) =
            entries.iter_mut().find(|(k, _)| k.as_str() == "inputs")
        {
            for input in inputs.iter_mut() {
                if let JsonValue::String(s) = input {
                    *s = swap_segment(s, source, target.name);
                }
            }
        }
    }
}

fn detect_source(plan: &JsonValue) -> Option<&'static str> {
    let invs = match plan {
        JsonValue::Object(e) => e
            .iter()
            .find(|(k, _)| k.as_str() == "invocations")
            .and_then(|(_, v)| match v {
                JsonValue::Array(a) => Some(a),
                _ => None,
            }),
        _ => None,
    }?;
    for inv in invs {
        if let JsonValue::Object(e) = inv {
            if let Some((_, JsonValue::Array(outs))) =
                e.iter().find(|(k, _)| k.as_str() == "outputs")
            {
                for out in outs {
                    if let JsonValue::String(s) = out {
                        if s.contains("/release/") {
                            return Some("release");
                        }
                        if s.contains("/debug/") {
                            return Some("debug");
                        }
                    }
                }
            }
        }
    }
    None
}

fn swap_segment(s: &str, source: &str, target: &str) -> String {
    if source == target {
        return s.to_string();
    }
    s.replace(&format!("/{source}/"), &format!("/{target}/"))
}

fn rewrite_invocation(inv: &mut JsonValue, source: &str, target: &ProfileSpec) {
    let entries = match inv {
        JsonValue::Object(e) => e,
        _ => return,
    };

    if let Some((_, JsonValue::String(prog))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "program")
    {
        *prog = swap_segment(prog, source, target.name);
    }

    if let Some((_, JsonValue::Array(args))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "args")
    {
        rewrite_args(args, source, target);
    }

    if let Some((_, JsonValue::Array(outputs))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "outputs")
    {
        for out in outputs.iter_mut() {
            if let JsonValue::String(s) = out {
                *s = swap_segment(s, source, target.name);
            }
        }
    }

    if let Some((_, JsonValue::Object(links))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "links")
    {
        let pairs: Vec<(String, JsonValue)> = links
            .iter()
            .map(|(k, v)| {
                let nk = swap_segment(k, source, target.name);
                let nv = match v {
                    JsonValue::String(s) => JsonValue::String(swap_segment(s, source, target.name)),
                    other => other.clone(),
                };
                (nk, nv)
            })
            .collect();
        links.clear();
        for (k, v) in pairs {
            links.insert(k, v);
        }
    }

    if let Some((_, JsonValue::String(cwd))) = entries.iter_mut().find(|(k, _)| k.as_str() == "cwd")
    {
        *cwd = swap_segment(cwd, source, target.name);
    }

    if let Some((_, JsonValue::Object(env))) = entries.iter_mut().find(|(k, _)| k.as_str() == "env")
    {
        for (k, v) in env.iter_mut() {
            if let JsonValue::String(s) = v {
                let new_val = match k.as_str() {
                    "PROFILE" => target.name.to_string(),
                    "OPT_LEVEL" => target.opt_level.to_string(),
                    "DEBUG" => target.debug_env.to_string(),
                    "DEBUG_ASSERTIONS" => target.debug_assertions.to_string(),
                    "OVERFLOW_CHECKS" => target.overflow_checks.to_string(),
                    _ => swap_segment(s, source, target.name),
                };
                *s = new_val;
            }
        }
    }
}

fn rewrite_args(args: &mut [JsonValue], source: &str, target: &ProfileSpec) {
    let mut i = 0;
    while i < args.len() {
        // Two-arg form: ["-C", "key=val"]
        if let JsonValue::String(s) = &args[i] {
            if s == "-C" && i + 1 < args.len() {
                if let JsonValue::String(next) = &args[i + 1] {
                    if let Some(rewritten) = rewrite_codegen_value(next, target) {
                        args[i + 1] = JsonValue::String(rewritten);
                    } else if let JsonValue::String(n) = &mut args[i + 1] {
                        *n = swap_segment(n, source, target.name);
                    }
                    i += 2;
                    continue;
                }
            }
            // One-arg form: "-C key=val"
            if let Some(rewritten) = rewrite_codegen_single(s, target) {
                args[i] = JsonValue::String(rewritten);
                i += 1;
                continue;
            }
        }
        if let JsonValue::String(s) = &mut args[i] {
            *s = swap_segment(s, source, target.name);
        }
        i += 1;
    }
}

fn rewrite_codegen_value(val: &str, target: &ProfileSpec) -> Option<String> {
    let (key, _) = val.split_once('=')?;
    let new_val = profile_codegen(key, target)?;
    Some(format!("{key}={new_val}"))
}

fn rewrite_codegen_single(s: &str, target: &ProfileSpec) -> Option<String> {
    let body = s.strip_prefix("-C ")?;
    let val = rewrite_codegen_value(body, target)?;
    Some(format!("-C {val}"))
}

fn profile_codegen(key: &str, target: &ProfileSpec) -> Option<&'static str> {
    match key {
        "opt-level" => Some(target.opt_level),
        "debuginfo" => Some(target.debuginfo),
        "debug-assertions" => Some(target.debug_assertions),
        "overflow-checks" => Some(target.overflow_checks),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn plan_with_paths(profile: &str) -> JsonValue {
        let json = format!(
            r#"{{
                "invocations": [
                    {{
                        "args": ["-C", "opt-level=3", "-C", "debuginfo=0", "--out-dir", "/work/target/{profile}/deps"],
                        "outputs": ["/work/target/{profile}/libfoo.rlib"],
                        "links": {{ "/work/target/{profile}/foo": "/work/target/{profile}/deps/foo-abc" }},
                        "cwd": "/work",
                        "env": {{ "PROFILE": "{profile}", "OPT_LEVEL": "3", "DEBUG": "false" }}
                    }}
                ],
                "inputs": ["/work/Cargo.toml"]
            }}"#,
        );
        json.parse().unwrap()
    }

    fn field<'a>(plan: &'a JsonValue, key: &str) -> &'a JsonValue {
        let entries = match plan {
            JsonValue::Object(e) => e,
            _ => panic!("expected object"),
        };
        let invs = entries
            .iter()
            .find(|(k, _)| k.as_str() == "invocations")
            .unwrap();
        let arr = match &invs.1 {
            JsonValue::Array(a) => a,
            _ => panic!("expected invocations array"),
        };
        let inv = match &arr[0] {
            JsonValue::Object(o) => o,
            _ => panic!("expected object invocation"),
        };
        &inv.iter().find(|(k, _)| k.as_str() == key).unwrap().1
    }

    fn args_strs(plan: &JsonValue) -> Vec<String> {
        match field(plan, "args") {
            JsonValue::Array(a) => a
                .iter()
                .filter_map(|v| match v {
                    JsonValue::String(s) => Some(s.clone()),
                    _ => None,
                })
                .collect(),
            _ => panic!(),
        }
    }

    #[test]
    fn release_to_debug() {
        let mut plan = plan_with_paths("release");
        rewrite_profile(&mut plan, &DEBUG);

        let strs = args_strs(&plan);
        assert!(strs.iter().any(|s| s == "opt-level=0"));
        assert!(strs.iter().any(|s| s == "debuginfo=2"));
        assert!(strs.iter().any(|s| s.contains("/debug/deps")));
        assert!(!strs.iter().any(|s| s.contains("/release/")));

        if let JsonValue::Array(outs) = field(&plan, "outputs") {
            if let JsonValue::String(s) = &outs[0] {
                assert!(s.contains("/debug/"));
                assert!(!s.contains("/release/"));
            }
        }

        if let JsonValue::Object(env) = field(&plan, "env") {
            let get = |key: &str| -> String {
                match env
                    .iter()
                    .find(|(k, _)| k.as_str() == key)
                    .unwrap()
                    .1
                    .clone()
                {
                    JsonValue::String(s) => s,
                    _ => panic!(),
                }
            };
            assert_eq!(get("PROFILE"), "debug");
            assert_eq!(get("OPT_LEVEL"), "0");
            assert_eq!(get("DEBUG"), "true");
        }
    }

    #[test]
    fn debug_to_release() {
        let mut plan = plan_with_paths("debug");
        rewrite_profile(&mut plan, &RELEASE);

        let strs = args_strs(&plan);
        assert!(strs.iter().any(|s| s == "opt-level=3"));
        assert!(strs.iter().any(|s| s == "debuginfo=0"));
        assert!(strs.iter().any(|s| s.contains("/release/deps")));
        assert!(!strs.iter().any(|s| s.contains("/debug/")));
    }

    #[test]
    fn idempotent() {
        let mut plan = plan_with_paths("release");
        let original = crate::patch::pretty_format(&plan);
        rewrite_profile(&mut plan, &RELEASE);
        let after = crate::patch::pretty_format(&plan);
        assert_eq!(original, after);
    }

    #[test]
    fn parse_profile_known() {
        assert!(parse_profile("release").is_some());
        assert!(parse_profile("debug").is_some());
        assert!(parse_profile("dev").is_none());
        assert!(parse_profile("").is_none());
    }
}
