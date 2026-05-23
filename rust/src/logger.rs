use std::io::IsTerminal;

#[derive(Clone)]
pub struct Logger {
    info_prefix: &'static str,
    warn_prefix: &'static str,
}

impl Logger {
    pub fn new() -> Self {
        let color = color_enabled();
        Self {
            info_prefix: if color { "\x1b[32m[INFO]\x1b[0m " } else { "" },
            warn_prefix: if color {
                "\x1b[33m[WARN]\x1b[0m "
            } else {
                "warning: "
            },
        }
    }

    pub fn info(&self, msg: &str) {
        eprintln!("{}{}", self.info_prefix, msg);
    }

    pub fn warn(&self, msg: &str) {
        eprintln!("{}{}", self.warn_prefix, msg);
    }
}

impl Default for Logger {
    fn default() -> Self {
        Self::new()
    }
}

fn color_enabled() -> bool {
    if std::env::var("NO_COLOR").is_ok() {
        return false;
    }
    std::io::stderr().is_terminal()
}
