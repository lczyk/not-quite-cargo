use std::collections::HashMap;

use tinyjson::JsonValue;

pub fn deep_replace(value: &JsonValue, replacements: &HashMap<String, String>) -> JsonValue {
    let keys = sorted_keys_longest_first(replacements);
    deep_replace_with_keys(value, replacements, &keys)
}

pub fn replace_string(s: &str, replacements: &HashMap<String, String>) -> String {
    let keys = sorted_keys_longest_first(replacements);
    replace_with(s, replacements, &keys)
}

fn deep_replace_with_keys(
    value: &JsonValue,
    replacements: &HashMap<String, String>,
    keys: &[String],
) -> JsonValue {
    match value {
        JsonValue::Object(entries) => {
            let mut out = HashMap::new();
            for (k, v) in entries {
                out.insert(
                    replace_with(k, replacements, keys),
                    deep_replace_with_keys(v, replacements, keys),
                );
            }
            JsonValue::Object(out)
        }
        JsonValue::Array(items) => {
            let out: Vec<JsonValue> = items
                .iter()
                .map(|v| deep_replace_with_keys(v, replacements, keys))
                .collect();
            JsonValue::Array(out)
        }
        JsonValue::String(s) => JsonValue::String(replace_with(s, replacements, keys)),
        other => other.clone(),
    }
}

fn replace_with(s: &str, replacements: &HashMap<String, String>, keys: &[String]) -> String {
    let mut out = s.to_string();
    for k in keys {
        if let Some(v) = replacements.get(k) {
            out = out.replace(k, v);
        }
    }
    out
}

fn sorted_keys_longest_first(m: &HashMap<String, String>) -> Vec<String> {
    let mut keys: Vec<String> = m.keys().cloned().collect();
    keys.sort_by(|a, b| b.len().cmp(&a.len()).then_with(|| a.cmp(b)));
    keys
}

#[cfg(test)]
mod tests {
    use super::*;

    fn replacements() -> HashMap<String, String> {
        let mut m = HashMap::new();
        m.insert("/project/root".to_string(), "{{PROJECT_ROOT}}".to_string());
        m.insert("/cargo/home".to_string(), "{{CARGO_HOME}}".to_string());
        m
    }

    #[test]
    fn replaces_in_string_value() {
        let v = JsonValue::String("/project/root/src/main.rs".to_string());
        let out = deep_replace(&v, &replacements());
        assert_eq!(
            out,
            JsonValue::String("{{PROJECT_ROOT}}/src/main.rs".to_string())
        );
    }

    #[test]
    fn replaces_in_object_keys() {
        let mut obj_map = HashMap::new();
        obj_map.insert(
            "/project/root/Cargo.toml".to_string(),
            JsonValue::String("input".to_string()),
        );
        let obj = JsonValue::Object(obj_map);
        let out = deep_replace(&obj, &replacements());
        if let JsonValue::Object(entries) = &out {
            assert!(entries.contains_key("{{PROJECT_ROOT}}/Cargo.toml"));
        } else {
            panic!("expected object");
        }
    }

    #[test]
    fn replaces_in_array() {
        let arr = JsonValue::Array(vec![
            JsonValue::String("/project/root/src/main.rs".to_string()),
            JsonValue::String("/cargo/home/bin/rustc".to_string()),
        ]);
        let out = deep_replace(&arr, &replacements());
        if let JsonValue::Array(items) = &out {
            assert_eq!(
                items[0],
                JsonValue::String("{{PROJECT_ROOT}}/src/main.rs".to_string())
            );
            assert_eq!(
                items[1],
                JsonValue::String("{{CARGO_HOME}}/bin/rustc".to_string())
            );
        } else {
            panic!("expected array");
        }
    }

    #[test]
    fn non_string_scalar_passes_through() {
        let v = JsonValue::Number(42.0);
        assert_eq!(deep_replace(&v, &replacements()), JsonValue::Number(42.0));
    }

    #[test]
    fn longest_key_wins_on_prefix_overlap() {
        let mut m = HashMap::new();
        m.insert("/project/root".to_string(), "{{ROOT}}".to_string());
        m.insert("/project/root/sub".to_string(), "{{SUB}}".to_string());
        let v = JsonValue::String("/project/root/sub/file.rs".to_string());
        let out = deep_replace(&v, &m);
        assert_eq!(out, JsonValue::String("{{SUB}}/file.rs".to_string()));
    }

    #[test]
    fn replace_string_standalone() {
        let s = "/project/root/src/main.rs";
        assert_eq!(
            replace_string(s, &replacements()),
            "{{PROJECT_ROOT}}/src/main.rs"
        );
    }
}
