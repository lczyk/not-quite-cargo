use std::collections::HashMap;
use std::fs;
use std::io::Write;
use std::path::Path;
use std::time::SystemTime;

use anyhow::Context;
use tinyjson::JsonValue;

#[derive(Clone, Debug)]
pub struct Invocation {
    pub number: i32,
    pub package_name: String,
    pub package_version: String,
    pub target_kind: Vec<String>,
    pub kind: Option<String>,
    pub compile_mode: String,
    pub deps: Vec<i32>,
    pub outputs: Vec<String>,
    pub links: HashMap<String, String>,
    pub program: String,
    pub args: Vec<String>,
    pub env: HashMap<String, String>,
    pub cwd: String,
}

pub fn load_plan_json(path: &Path) -> anyhow::Result<JsonValue> {
    let data =
        fs::read_to_string(path).with_context(|| format!("read build plan: {}", path.display()))?;

    let plan: JsonValue = data
        .parse()
        .with_context(|| format!("parse build plan: {}", path.display()))?;

    match &plan {
        JsonValue::Object(entries) => {
            let has_invocations = entries
                .iter()
                .any(|(k, v)| k == "invocations" && matches!(v, JsonValue::Array(_)));
            if !has_invocations {
                anyhow::bail!(
                    "{} does not look like a Cargo build plan (no invocations array)",
                    path.display()
                );
            }
        }
        _ => anyhow::bail!(
            "{} does not look like a Cargo build plan (not an object)",
            path.display()
        ),
    }

    Ok(plan)
}

pub fn decode_invocations(raw: &[JsonValue]) -> anyhow::Result<Vec<Invocation>> {
    let mut invs = Vec::with_capacity(raw.len());
    for (i, item) in raw.iter().enumerate() {
        let inv = invocation_from_json(item, i)?;
        invs.push(inv);
    }
    Ok(invs)
}

pub fn write_atomic(path: &Path, data: &[u8], perm: std::fs::Permissions) -> anyhow::Result<()> {
    let dir = path.parent().unwrap_or_else(|| Path::new("."));

    let seed = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0);
    let pid = std::process::id();
    let tmp_name = dir.join(format!(".nqc-patch-{pid:x}-{seed:x}"));

    let mut f = fs::File::create(&tmp_name)
        .with_context(|| format!("create temp file: {}", tmp_name.display()))?;
    f.write_all(data)
        .with_context(|| format!("write temp file: {}", tmp_name.display()))?;
    f.set_permissions(perm)
        .with_context(|| format!("chmod temp file: {}", tmp_name.display()))?;
    drop(f);

    fs::rename(&tmp_name, path)
        .with_context(|| format!("rename {} -> {}", tmp_name.display(), path.display()))?;

    Ok(())
}

fn invocation_from_json(v: &JsonValue, number: usize) -> anyhow::Result<Invocation> {
    let obj = match v {
        JsonValue::Object(entries) => entries,
        _ => anyhow::bail!("invocation {number} has unexpected shape"),
    };

    let get_str = |key: &str| -> anyhow::Result<&str> {
        obj.iter()
            .find(|(k, _)| k.as_str() == key)
            .and_then(|(_, v)| match v {
                JsonValue::String(s) => Some(s.as_str()),
                _ => None,
            })
            .ok_or_else(|| {
                anyhow::anyhow!("invocation {number}: missing or non-string field '{key}'")
            })
    };

    let get_str_opt = |key: &str| -> Option<&str> {
        obj.iter()
            .find(|(k, _)| k.as_str() == key)
            .and_then(|(_, v)| match v {
                JsonValue::String(s) => Some(s.as_str()),
                _ => None,
            })
    };

    let get_strings = |key: &str| -> Vec<String> {
        obj.iter()
            .find(|(k, _)| k.as_str() == key)
            .map(|(_, v)| match v {
                JsonValue::Array(arr) => arr
                    .iter()
                    .filter_map(|item| match item {
                        JsonValue::String(s) => Some(s.clone()),
                        _ => None,
                    })
                    .collect(),
                _ => Vec::new(),
            })
            .unwrap_or_default()
    };

    let get_ints = |key: &str| -> Vec<i32> {
        obj.iter()
            .find(|(k, _)| k.as_str() == key)
            .map(|(_, v)| match v {
                JsonValue::Array(arr) => arr
                    .iter()
                    .filter_map(|item| match item {
                        JsonValue::Number(n) => Some(*n as i32),
                        _ => None,
                    })
                    .collect(),
                _ => Vec::new(),
            })
            .unwrap_or_default()
    };

    let get_map = |key: &str| -> HashMap<String, String> {
        let mut m = HashMap::new();
        if let Some((_, JsonValue::Object(entries))) = obj.iter().find(|(k, _)| k.as_str() == key) {
            for (k, v) in entries {
                if let JsonValue::String(s) = v {
                    m.insert(k.clone(), s.clone());
                }
            }
        }
        m
    };

    let kind = match obj.iter().find(|(k, _)| k.as_str() == "kind") {
        Some((_, JsonValue::Null)) => None,
        Some((_, JsonValue::String(s))) => Some(s.clone()),
        _ => None,
    };

    Ok(Invocation {
        number: number as i32,
        package_name: get_str("package_name")?.to_string(),
        package_version: get_str("package_version")?.to_string(),
        target_kind: get_strings("target_kind"),
        kind,
        compile_mode: get_str_opt("compile_mode").unwrap_or("build").to_string(),
        deps: get_ints("deps"),
        outputs: get_strings("outputs"),
        links: get_map("links"),
        program: get_str_opt("program").unwrap_or("").to_string(),
        args: get_strings("args"),
        env: get_map("env"),
        cwd: get_str_opt("cwd").unwrap_or("").to_string(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn load_plan_rejects_non_object() {
        let dir = std::env::temp_dir();
        let path = dir.join("test_plan_array.json");
        fs::write(&path, "[]").unwrap();
        let result = load_plan_json(&path);
        assert!(result.is_err());
        let _ = fs::remove_file(&path);
    }

    #[test]
    fn load_plan_rejects_object_without_invocations() {
        let dir = std::env::temp_dir();
        let path = dir.join("test_plan_no_inv.json");
        fs::write(&path, r#"{"inputs": []}"#).unwrap();
        let result = load_plan_json(&path);
        assert!(result.is_err());
        let _ = fs::remove_file(&path);
    }

    #[test]
    fn decode_invocation_from_json() {
        let json: JsonValue = r#"{
            "package_name": "hello",
            "package_version": "0.1.0",
            "target_kind": ["bin"],
            "kind": null,
            "compile_mode": "build",
            "deps": [],
            "outputs": ["/out/hello"],
            "links": {},
            "program": "rustc",
            "args": ["--edition=2021", "/src/main.rs"],
            "env": {"CARGO_PKG_NAME": "hello"},
            "cwd": "/proj"
        }"#
        .parse()
        .unwrap();

        let inv = invocation_from_json(&json, 0).unwrap();
        assert_eq!(inv.package_name, "hello");
        assert_eq!(inv.program, "rustc");
        assert!(inv.kind.is_none());
        assert_eq!(inv.args.len(), 2);
        assert_eq!(inv.env.get("CARGO_PKG_NAME").unwrap(), "hello");
    }
}
