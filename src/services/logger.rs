use chrono::Local;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::sync::Mutex;
use tokio::sync::broadcast;

#[derive(Clone, Debug)]
pub enum LogLevel {
    Debug,
    Info,
    Warn,
    Error,
}

impl LogLevel {
    pub fn as_str(&self) -> &'static str {
        match self {
            LogLevel::Debug => "DEBUG",
            LogLevel::Info => "INFO",
            LogLevel::Warn => "WARN",
            LogLevel::Error => "ERROR",
        }
    }
}

#[derive(Clone, Debug)]
pub struct LogEntry {
    pub level: LogLevel,
    pub component: String,
    pub message: String,
    pub timestamp: String,
}

impl LogEntry {
    pub fn new(level: LogLevel, component: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            level,
            component: component.into(),
            message: message.into(),
            timestamp: Local::now().format("%Y-%m-%d %H:%M:%S").to_string(),
        }
    }

    pub fn display(&self) -> String {
        format!(
            "[{}][{}] {}",
            self.component,
            self.level.as_str(),
            self.message
        )
    }
}

pub struct Logger {
    log_dir: PathBuf,
    current_file: Mutex<Option<String>>,
    sender: broadcast::Sender<LogEntry>,
}

impl Logger {
    pub fn new() -> Self {
        let home = std::env::var("HOME").unwrap_or_else(|_| ".".to_string());
        let log_dir = PathBuf::from(home).join(".aura").join("log");

        if let Err(e) = fs::create_dir_all(&log_dir) {
            eprintln!("Failed to create log directory: {}", e);
        }

        let (sender, _) = broadcast::channel(1000);

        Self {
            log_dir,
            current_file: Mutex::new(None),
            sender,
        }
    }

    fn get_log_file_path(&self) -> PathBuf {
        let date = Local::now().format("%d-%m-%Y").to_string();
        self.log_dir.join(format!("{}.txt", date))
    }

    fn ensure_log_file(&self) -> Option<std::fs::File> {
        let file_path = self.get_log_file_path();
        let date = Local::now().format("%d-%m-%Y").to_string();

        {
            let mut current = self.current_file.lock().unwrap();
            if current.as_ref() == Some(&date) {
                return OpenOptions::new().append(true).open(&file_path).ok();
            }
            *current = Some(date);
        }

        OpenOptions::new()
            .create(true)
            .append(true)
            .open(&file_path)
            .ok()
    }

    pub fn log(&self, level: LogLevel, component: impl Into<String>, message: impl Into<String>) {
        let entry = LogEntry::new(level, component, message);

        if let Some(mut file) = self.ensure_log_file() {
            let _ = writeln!(
                file,
                "{} [{}][{}] {}",
                entry.timestamp,
                entry.component,
                entry.level.as_str(),
                entry.message
            );
        }

        let _ = self.sender.send(entry);
    }

    pub fn info(&self, component: impl Into<String>, message: impl Into<String>) {
        self.log(LogLevel::Info, component, message);
    }

    pub fn debug(&self, component: impl Into<String>, message: impl Into<String>) {
        self.log(LogLevel::Debug, component, message);
    }

    pub fn warn(&self, component: impl Into<String>, message: impl Into<String>) {
        self.log(LogLevel::Warn, component, message);
    }

    pub fn error(&self, component: impl Into<String>, message: impl Into<String>) {
        self.log(LogLevel::Error, component, message);
    }

    pub fn subscribe(&self) -> broadcast::Receiver<LogEntry> {
        self.sender.subscribe()
    }
}

impl Default for Logger {
    fn default() -> Self {
        Self::new()
    }
}
