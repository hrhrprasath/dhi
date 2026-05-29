use crate::prompt::synthesis_prompt;

pub struct Summarizer {
    client: reqwest::Client,
    model: String,
    endpoint: String,
}

impl Clone for Summarizer {
    fn clone(&self) -> Self {
        Self {
            client: reqwest::Client::new(),
            model: self.model.clone(),
            endpoint: self.endpoint.clone(),
        }
    }
}

impl Summarizer {
    pub fn new(model: String, endpoint: String) -> Self {
        Self {
            client: reqwest::Client::new(),
            model,
            endpoint,
        }
    }

    pub async fn summarize(
        &self,
        raw_data: &str,
        pipeline: &str,
        user_input: &str,
        conversation_context: &str,
    ) -> String {
        let prompt = synthesis_prompt(raw_data, pipeline, user_input, conversation_context);

        let request = serde_json::json!({
            "model": self.model,
            "prompt": prompt,
            "stream": false
        });

        match self.client
            .post(format!("{}/api/generate", self.endpoint))
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
        Self::new(
            "qwen2.5:1.5b".to_string(),
            "http://localhost:11434".to_string(),
        )
    }
}