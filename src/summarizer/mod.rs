use crate::prompt::synthesis_prompt;

const OLLAMA_BASE_URL: &str = "http://localhost:11434";
const DEFAULT_MODEL: &str = "qwen2.5:1.5b";

pub struct Summarizer {
    client: reqwest::Client,
    model: String,
}

impl Clone for Summarizer {
    fn clone(&self) -> Self {
        Self {
            client: reqwest::Client::new(),
            model: self.model.clone(),
        }
    }
}

impl Summarizer {
    pub fn new() -> Self {
        Self {
            client: reqwest::Client::new(),
            model: DEFAULT_MODEL.to_string(),
        }
    }

    #[allow(dead_code)]
    pub fn with_model(model: String) -> Self {
        Self {
            client: reqwest::Client::new(),
            model,
        }
    }

    pub async fn summarize(&self, raw_data: &str, pipeline: &str, custom_prompt: Option<&str>) -> String {
        let prompt = if let Some(custom) = custom_prompt {
            format!("{}\n\nRaw data:\n{}\n\nSummary:", custom, raw_data)
        } else {
            synthesis_prompt(raw_data, pipeline)
        };

        let request = serde_json::json!({
            "model": self.model,
            "prompt": prompt,
            "stream": false
        });

        match self.client
            .post(format!("{}/api/generate", OLLAMA_BASE_URL))
            .json(&request)
            .send()
            .await
        {
            Ok(response) => {
                let status = response.status();
                if status.is_success() {
                    if let Ok(ollama_resp) = response.json::<OllamaResponse>().await {
                        return ollama_resp.response.trim().to_string();
                    }
                }
                format!("[Summary unavailable - API returned status {}]", status.as_u16())
            }
            Err(e) => format!("[Summary unavailable - {}]", e)
        }
    }
}

#[derive(serde::Deserialize)]
struct OllamaResponse {
    response: String,
}

impl Default for Summarizer {
    fn default() -> Self {
        Self::new()
    }
}