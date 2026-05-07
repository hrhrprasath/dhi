use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use uuid::Uuid;

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum QueueItemStatus {
    Pending,
    Processing,
    Completed,
    Failed,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Intent {
    pub pipeline: String,
    pub action: String,
    pub params: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InputQueueItem {
    pub id: String,
    pub raw_input: String,
    pub intent: Option<Intent>,
    pub status: QueueItemStatus,
}

impl InputQueueItem {
    pub fn new(raw_input: String) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            raw_input,
            intent: None,
            status: QueueItemStatus::Pending,
        }
    }

    pub fn with_intent(mut self, intent: Intent) -> Self {
        self.intent = Some(intent);
        self
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutputQueueItem {
    pub id: String,
    pub pipeline: String,
    pub timestamp: DateTime<Utc>,
    pub input_id: String,
    pub raw_data: String,
    pub summary: String,
    pub status: QueueItemStatus,
}

impl OutputQueueItem {
    pub fn new(pipeline: String, input_id: String, raw_data: String, summary: String) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            pipeline,
            timestamp: Utc::now(),
            input_id,
            raw_data,
            summary,
            status: QueueItemStatus::Completed,
        }
    }
}

pub struct QueueManager {
    data_dir: PathBuf,
    output_queue_path: PathBuf,
    input_queue: Vec<InputQueueItem>,
}

impl QueueManager {
    pub fn new() -> anyhow::Result<Self> {
        let data_dir = std::env::current_dir()
            .map(|p| p.join("data"))
            .unwrap_or_else(|_| PathBuf::from("./data"));
        let queue_dir = data_dir.join("queue");

        let _ = fs::create_dir_all(&queue_dir);
        let _ = fs::create_dir_all(&data_dir);

        Ok(Self {
            data_dir,
            output_queue_path: queue_dir.join("output.json"),
            input_queue: Vec::new(),
        })
    }

    fn load_json<T: for<'de> Deserialize<'de> + Default>(path: &PathBuf) -> T {
        match fs::read_to_string(path) {
            Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
            Err(_) => T::default(),
        }
    }

    fn save_json<T: Serialize + ?Sized>(path: &PathBuf, data: &T) -> anyhow::Result<()> {
        let content = serde_json::to_string_pretty(data)?;
        fs::write(path, content)?;
        Ok(())
    }

    pub fn enqueue_input(&mut self, item: InputQueueItem) -> String {
        let id = item.id.clone();
        self.input_queue.push(item);
        id
    }

    pub fn get_pending_inputs(&self) -> Vec<InputQueueItem> {
        self.input_queue
            .iter()
            .filter(|item| {
                matches!(
                    item.status,
                    QueueItemStatus::Pending | QueueItemStatus::Failed
                )
            })
            .cloned()
            .collect()
    }

    pub fn update_input_status(&mut self, id: &str, status: QueueItemStatus) {
        if let Some(item) = self.input_queue.iter_mut().find(|i| i.id == id) {
            item.status = status;
        }
    }

    pub fn load_output_queue(&self) -> Vec<OutputQueueItem> {
        Self::load_json(&self.output_queue_path)
    }

    pub fn save_output_queue(&self, queue: &[OutputQueueItem]) -> anyhow::Result<()> {
        Self::save_json(&self.output_queue_path, queue)
    }

    pub fn enqueue_output(&self, item: OutputQueueItem) -> anyhow::Result<String> {
        let mut queue = self.load_output_queue();
        let id = item.id.clone();
        queue.push(item);
        self.save_output_queue(&queue)?;

        if let Some(last) = queue.last() {
            let _ = self.persist_as_markdown(last);
        }

        Ok(id)
    }

    fn persist_as_markdown(&self, item: &OutputQueueItem) -> anyhow::Result<()> {
        let pipeline_dir = self.data_dir.join(&item.pipeline);
        fs::create_dir_all(&pipeline_dir)?;

        let filename = item.timestamp.format("%Y-%m-%dT%H-%M-%S").to_string();
        let md_path = pipeline_dir.join(format!("{}.md", filename));

        let content = format!(
            "# {} - {}\n\n**Pipeline:** {}\n**Time:** {}\n\n## Summary\n{}\n\n## Raw Data\n```\n{}\n```",
            item.pipeline,
            item.id,
            item.pipeline,
            item.timestamp.to_rfc3339(),
            item.summary,
            item.raw_data
        );

        fs::write(md_path, content)?;
        Ok(())
    }

    pub fn get_recent_outputs(&self, limit: usize) -> Vec<OutputQueueItem> {
        let queue = self.load_output_queue();
        queue.into_iter().rev().take(limit).collect()
    }

    pub fn input_queue_len(&self) -> usize {
        self.input_queue.len()
    }

    pub fn output_queue_len(&self) -> usize {
        let queue = self.load_output_queue();
        queue.len()
    }
}

impl Default for QueueManager {
    fn default() -> Self {
        Self {
            data_dir: PathBuf::from("./data"),
            output_queue_path: PathBuf::from("./data/queue/output.json"),
            input_queue: Vec::new(),
        }
    }
}
