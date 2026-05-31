use std::collections::HashSet;

use crate::plan::Invocation;

pub fn resolve_invocation_order(invs: &[Invocation]) -> anyhow::Result<Vec<Invocation>> {
    let known: HashSet<i32> = invs.iter().map(|inv| inv.number).collect();
    for inv in invs {
        for &dep in &inv.deps {
            if !known.contains(&dep) {
                anyhow::bail!(
                    "invocation {} depends on unknown invocation {dep}",
                    inv.number
                );
            }
        }
    }

    let mut pending = invs.to_vec();
    pending.sort_by_key(|inv| inv.number);

    let mut satisfied: HashSet<i32> = HashSet::new();
    let mut ordered = Vec::with_capacity(invs.len());

    while !pending.is_empty() {
        let idx = pending
            .iter()
            .position(|inv| inv.deps.iter().all(|dep| satisfied.contains(dep)));

        match idx {
            Some(i) => {
                let inv = pending.remove(i);
                satisfied.insert(inv.number);
                ordered.push(inv);
            }
            None => {
                let remaining: Vec<i32> = pending.iter().map(|inv| inv.number).collect();
                anyhow::bail!("circular dependency among invocations {remaining:?}");
            }
        }
    }

    Ok(ordered)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn inv(number: i32, deps: &[i32]) -> Invocation {
        Invocation {
            number,
            package_name: format!("pkg{number}"),
            package_version: "0.1.0".to_string(),
            target_kind: vec![],
            kind: None,
            compile_mode: "build".to_string(),
            deps: deps.to_vec(),
            outputs: vec![],
            links: HashMap::new(),
            program: "rustc".to_string(),
            args: vec![],
            env: HashMap::new(),
            cwd: "/".to_string(),
        }
    }

    #[test]
    fn linear_chain() {
        let invs = vec![inv(0, &[]), inv(1, &[0]), inv(2, &[1])];
        let ordered = resolve_invocation_order(&invs).unwrap();
        let nums: Vec<i32> = ordered.iter().map(|i| i.number).collect();
        assert_eq!(nums, vec![0, 1, 2]);
    }

    #[test]
    fn parallel_ready() {
        let invs = vec![inv(0, &[]), inv(1, &[]), inv(2, &[0, 1])];
        let ordered = resolve_invocation_order(&invs).unwrap();
        let nums: Vec<i32> = ordered.iter().map(|i| i.number).collect();
        // 0 and 1 are both ready first, smallest number wins.
        assert_eq!(nums[0], 0);
        assert_eq!(nums[1], 1);
        assert_eq!(nums[2], 2);
    }

    #[test]
    fn circular_dependency() {
        let invs = vec![inv(0, &[1]), inv(1, &[0])];
        assert!(resolve_invocation_order(&invs).is_err());
    }

    #[test]
    fn unknown_dependency() {
        let invs = vec![inv(0, &[99])];
        assert!(resolve_invocation_order(&invs).is_err());
    }

    #[test]
    fn deterministic_on_tie() {
        let invs = vec![inv(2, &[]), inv(1, &[]), inv(0, &[])];
        let ordered = resolve_invocation_order(&invs).unwrap();
        let nums: Vec<i32> = ordered.iter().map(|i| i.number).collect();
        assert_eq!(nums, vec![0, 1, 2]);
    }
}
