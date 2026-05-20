use crate::prompt::intent_prompt;
use crate::services::Logger;
use anyhow::{Result, anyhow};
use chrono::Utc;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use std::sync::Arc;

const OLLAMA_BASE_URL: &str = "http://localhost:11434";
const DEFAULT_MODEL: &str = "qwen2.5:1.5b";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentRequest {
    pub model: String,
    pub prompt: String,
    pub stream: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentResponse {
    pub model: String,
    pub response: String,
    pub done: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParsedIntent {
    pub event: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub params: Option<serde_json::Value>,
}

pub struct IntentRouter {
    client: Client,
    model: String,
    logger: Arc<Logger>,
}

impl IntentRouter {
    pub fn new(model: Option<String>, logger: Arc<Logger>) -> Self {
        let model = model.unwrap_or_else(|| DEFAULT_MODEL.to_string());
        logger.info("ROUTER", "Intent Router initialized");
        Self {
            client: Client::new(),
            model,
            logger,
        }
    }

    pub async fn route(&self, user_input: &str) -> Result<ParsedIntent> {
        self.logger.info("ROUTER", &format!("User input: {}", user_input));
        let prompt = intent_prompt(user_input);
        
        self.save_to_file(&prompt);

        let request = IntentRequest {
            model: self.model.clone(),
            prompt,
            stream: false,
            format: Some(serde_json::json!({"type": "object"})),
        };

        let response = self.client
            .post(format!("{}/api/generate", OLLAMA_BASE_URL))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            let err = format!("Ollama request failed: {}", response.status());
            self.logger.error("ROUTER", &err);
            return Err(anyhow!(err));
        }

        let ollama_resp: IntentResponse = response.json().await?;
        
        self.logger.debug("ROUTER", &format!("Ollama raw response:\n{}", ollama_resp.response));
        
        let parsed = self.parse_json_response(&ollama_resp.response)?;
        
        self.logger.info("ROUTER", &format!("Intent parsed: {:?}", parsed));

        Ok(parsed)
    }

    fn parse_json_response(&self, response: &str) -> Result<ParsedIntent> {
        let trimmed = response.trim();
        let json_str = if trimmed.starts_with("```json") {
            trimmed.trim_start_matches("```json").trim_end_matches("```").trim()
        } else if trimmed.starts_with("```") {
            trimmed.trim_start_matches("```").trim_end_matches("```").trim()
        } else {
            trimmed
        };

        let parsed: serde_json::Value = serde_json::from_str(json_str)
            .map_err(|e| {
                let err = format!("Failed to parse JSON: {} - Response: {}", e, json_str);
                self.logger.error("ROUTER", &err);
                anyhow!(err)
            })?;

        Ok(ParsedIntent {
            event: parsed["event"]
                .as_str()
                .ok_or_else(|| anyhow!("Missing 'event' field"))?
                .to_string(),
            params: parsed.get("params").cloned(),
        })
    }

    pub async fn health_check(&self) -> bool {
        match self.client.get(format!("{}/api/tags", OLLAMA_BASE_URL)).send().await {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }

    fn save_to_file(&self, prompt: &str) {
        let data_dir = PathBuf::from("./.data/intent");
        let _ = fs::create_dir_all(&data_dir);
        
        let timestamp = Utc::now().format("%Y-%m-%dT%H-%M-%S").to_string();
        let filename = format!("{}.md", timestamp);
        let path = data_dir.join(&filename);
        
        let content = format!("# Intent Prompt\n\n{}", prompt);
        let _ = fs::write(&path, content);
    }
}

impl Default for IntentRouter {
    fn default() -> Self {
        Self::new(None, Arc::new(Logger::new()))
    }
}