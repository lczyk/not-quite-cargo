use std::collections::HashMap;

use tinyjson::JsonValue;

use crate::deepreplace;

const STRIPPED_ENV_KEYS: &[&str] = &["CARGO", "PROJECT_ROOT", "CARGO_HOME", "RUSTC"];

pub fn patch_plan(
    plan: &JsonValue,
    project_root: &str,
    cargo_home: &str,
) -> anyhow::Result<JsonValue> {
    if project_root.is_empty() {
        anyhow::bail!("PatchPlan: projectRoot is required");
    }
    if cargo_home.is_empty() {
        anyhow::bail!("PatchPlan: cargoHome is required");
    }

    let mut reverse = HashMap::new();
    reverse.insert(project_root.to_string(), "{{PROJECT_ROOT}}".to_string());
    reverse.insert(cargo_home.to_string(), "{{CARGO_HOME}}".to_string());

    let mut out = plan.clone();

    let invocations_raw = match plan {
        JsonValue::Object(entries) => entries
            .iter()
            .find(|(k, _)| k.as_str() == "invocations")
            .and_then(|(_, v)| match v {
                JsonValue::Array(arr) => Some(arr.clone()),
                _ => None,
            }),
        _ => None,
    };

    if let Some(invs_raw) = invocations_raw {
        let patched: Vec<JsonValue> = invs_raw
            .iter()
            .map(|inv| {
                let mut clone = inv.clone();
                patch_invocation(&mut clone);
                deepreplace::deep_replace(&clone, &reverse)
            })
            .collect();

        if let JsonValue::Object(ref mut entries) = out {
            if let Some((_, v)) = entries
                .iter_mut()
                .find(|(k, _)| k.as_str() == "invocations")
            {
                *v = JsonValue::Array(patched);
            }
        }
    }

    // Patch inputs array (array of strings).
    if let JsonValue::Object(ref mut entries) = out {
        if let Some((_, JsonValue::Array(inputs))) =
            entries.iter_mut().find(|(k, _)| k.as_str() == "inputs")
        {
            let patched_inputs: Vec<JsonValue> = inputs
                .iter()
                .map(|input| match input {
                    JsonValue::String(s) => {
                        JsonValue::String(deepreplace::replace_string(s, &reverse))
                    }
                    other => other.clone(),
                })
                .collect();
            *inputs = patched_inputs;
        }
    }

    Ok(out)
}

fn patch_invocation(inv: &mut JsonValue) {
    let entries = match inv {
        JsonValue::Object(entries) => entries,
        _ => return,
    };

    // Replace program="rustc" with placeholder.
    if let Some((_, JsonValue::String(prog))) =
        entries.iter().find(|(k, _)| k.as_str() == "program")
    {
        if prog == "rustc" {
            if let Some((_, v)) = entries.iter_mut().find(|(k, _)| k.as_str() == "program") {
                *v = JsonValue::String("{{RUSTC}}".to_string());
            }
        }
    }

    // Strip injected env keys.
    if let Some((_, JsonValue::Object(env_entries))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "env")
    {
        env_entries.retain(|k, _| !STRIPPED_ENV_KEYS.iter().any(|strip| strip == k));
    }

    // Drop --diagnostic-width and its value from args.
    if let Some((_, JsonValue::Array(args))) =
        entries.iter_mut().find(|(k, _)| k.as_str() == "args")
    {
        let mut filtered = Vec::with_capacity(args.len());
        let mut skip_next = false;
        for arg in args.drain(..) {
            if skip_next {
                skip_next = false;
                continue;
            }
            if let JsonValue::String(s) = &arg {
                if s == "--diagnostic-width" {
                    skip_next = true; // two-arg form: skip value too
                    continue;
                }
                if s.starts_with("--diagnostic-width=") {
                    continue;
                }
            }
            filtered.push(arg);
        }
        *args = filtered;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn load_test_plan() -> JsonValue {
        let data = include_str!("testdata/plan.small.json");
        data.parse().unwrap()
    }

    fn load_golden() -> JsonValue {
        let data = include_str!("testdata/plan.small.patched.json");
        data.parse().unwrap()
    }

    #[test]
    fn patch_produces_golden_output() {
        let plan = load_test_plan();
        let patched = patch_plan(&plan, "/proj/root", "/cargo/home").unwrap();
        let golden = load_golden();

        // Compare formatted JSON strings for byte-identical check.
        let patched_str = pretty_format(&patched);
        let golden_str = pretty_format(&golden);
        assert_eq!(patched_str, golden_str);
    }

    #[test]
    fn patch_requires_project_root() {
        let plan = load_test_plan();
        let result = patch_plan(&plan, "", "/cargo/home");
        assert!(result.is_err());
    }

    #[test]
    fn patch_requires_cargo_home() {
        let plan = load_test_plan();
        let result = patch_plan(&plan, "/proj/root", "");
        assert!(result.is_err());
    }
}

/// Pretty-print JsonValue with 4-space indent, keys sorted alphabetically.
pub fn pretty_format(value: &JsonValue) -> String {
    format_value(value, 0)
}

fn format_value(value: &JsonValue, indent: usize) -> String {
    match value {
        JsonValue::Object(entries) => {
            if entries.is_empty() {
                return "{}".to_string();
            }
            let pad = " ".repeat(indent + 4);
            let close_pad = " ".repeat(indent);
            let mut keys: Vec<&String> = entries.keys().collect();
            keys.sort();
            let mut parts = Vec::new();
            for key in keys {
                let val = &entries[key];
                let formatted_val = format_value(val, indent + 4);
                parts.push(format!("{pad}\"{key}\": {formatted_val}"));
            }
            format!("{{\n{}\n{close_pad}}}", parts.join(",\n"))
        }
        JsonValue::Array(items) => {
            if items.is_empty() {
                return "[]".to_string();
            }
            let pad = " ".repeat(indent + 4);
            let close_pad = " ".repeat(indent);
            let parts: Vec<String> = items
                .iter()
                .map(|item| format!("{pad}{}", format_value(item, indent + 4)))
                .collect();
            format!("[\n{}\n{close_pad}]", parts.join(",\n"))
        }
        JsonValue::Null => "null".to_string(),
        JsonValue::Boolean(b) => b.to_string(),
        JsonValue::Number(n) => {
            // Format without trailing .0 for integers.
            if n.fract() == 0.0 && n.is_finite() {
                format!("{}", *n as i64)
            } else {
                format!("{n}")
            }
        }
        JsonValue::String(s) => {
            // JSON-escape the string.
            json_escape(s)
        }
    }
}

fn json_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if c.is_control() => {
                out.push_str(&format!("\\u{:04x}", c as u32));
            }
            c => out.push(c),
        }
    }
    out.push('"');
    out
}
