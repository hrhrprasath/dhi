use crate::AppState;
use ratatui::{
    layout::Rect,
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

const ANIMATION_FRAMES: &[&str; 12] = &[
    "        \n        \n        \n    .   \n        \n        \n        \n        ",
    "        \n        \n    ·   \n   ·.·  \n    ·   \n        \n        \n        ",
    "        \n    .   \n   · ·  \n  · o · \n   · ·  \n    .   \n        \n        ",
    "    ·   \n  ·   · \n ·     · \n·   o   ·\n ·     · \n  ·   · \n    ·   \n        ",
    "   · ·  \n  · · · \n · ·o· · \n· ·o·o· ·\n · ·o· · \n  · · · \n   · ·  \n        ",
    "  · · · \n · · · · \n· · o · ·\n· ·oOo· ·\n· · o · ·\n · · · · \n  · · · \n        ",
    " ·  ·  · \n·  · ·  ·\n · · · · \n· · O · ·\n · · · · \n·  · ·  ·\n ·  ·  · \n        ",
    "·   ·   ·\n ·  ·  · \n· · O · ·\n· O o O ·\n· · O · ·\n ·  ·  · \n·   ·   ·\n        ",
    " ·  ·  · \n·  · ·  ·\n · · · · \n· · O · ·\n · · · · \n·  · ·  ·\n ·  ·  · \n        ",
    "        \n  · · · \n · · · · \n · ·o· · \n · · · · \n  · · · \n        \n        ",
    "        \n        \n   · ·  \n  · · · \n   · ·  \n        \n        \n        ",
    "        \n        \n        \n    ·   \n        \n        \n        \n        ",
];

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let metrics_text = vec![
        Line::from(format!("CPU: [||||||  ] {}%", state.cpu_usage)),
        Line::from(format!("RAM: [||||||||||] {}GB", state.ram_usage_gb)),
        Line::from(""),
        Line::from("NVIDIA API"),
        Line::from(format!("Calls: {} / min", state.nvidia_calls_per_min)),
        Line::from(""),
        Line::from(format!("[Ollama: {}]", state.router_status)),
    ];
    let metrics_block = Block::default()
        .borders(Borders::ALL)
        .title(" SYSTEM METRICS ");
    let metrics = Paragraph::new(metrics_text).block(metrics_block);
    f.render_widget(metrics, area);
}

pub fn get_animation_frame(frame_idx: usize) -> &'static str {
    ANIMATION_FRAMES[frame_idx % 12]
}
