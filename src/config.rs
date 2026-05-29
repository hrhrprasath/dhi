use serde::Deserialize;
use std::path::PathBuf;

#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    pub llm: LlmConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct LlmConfig {
    #[serde(default = "default_endpoint")]
    pub endpoint: String,
    #[serde(default = "default_model")]
    pub model: String,
}

impl Default for LlmConfig {
    fn default() -> Self {
        Self {
            endpoint: default_endpoint(),
            model: default_model(),
        }
    }
}

fn default_endpoint() -> String {
    "http://localhost:11434".to_string()
}

fn default_model() -> String {
    "qwen2.5:1.5b".to_string()
}

impl Config {
    pub fn load() -> Self {
        let paths = [
            PathBuf::from("config/config.yaml"),
            PathBuf::from("config.yaml"),
        ];
        for path in &paths {
            if let Ok(content) = std::fs::read_to_string(path) {
                if let Ok(cfg) = serde_yaml::from_str(&content) {
                    return cfg;
                }
            }
        }
        Config::default()
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            llm: LlmConfig {
                endpoint: default_endpoint(),
                model: default_model(),
            },
        }
    }
}
