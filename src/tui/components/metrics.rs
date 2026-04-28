use crate::AppState;
use ratatui::{
    layout::Rect,
    style::{Color, Style},
    text::{Line, Span},
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

fn get_bar_color(value: u16) -> Color {
    if value < 50 {
        Color::Green
    } else if value < 80 {
        Color::Yellow
    } else {
        Color::Red
    }
}

fn make_bar(value: u16, total: u16) -> String {
    let percent = if total > 0 {
        (value as f32 / total as f32 * 100.0) as u16
    } else {
        0
    };
    let filled = (percent as f32 / 6.25).round() as usize;
    let empty = 16 - filled;
    format!("{}{:empty$} {}%", "█".repeat(filled), "", percent)
}

fn get_status_indicator(online: bool, processing: bool) -> (String, Color) {
    if processing {
        ("●".to_string(), Color::Yellow)
    } else if online {
        ("●".to_string(), Color::Green)
    } else {
        ("○".to_string(), Color::Gray)
    }
}

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let cpu_bar = make_bar(state.cpu_usage, 100);
    let cpu_color = get_bar_color(state.cpu_usage);
    let ram_percent = if state.ram_total_gb > 0 {
        ((state.ram_usage_gb as f32 / state.ram_total_gb as f32) * 100.0) as u16
    } else {
        0
    };
    let ram_bar = make_bar(state.ram_usage_gb, state.ram_total_gb);
    let ram_color = get_bar_color(ram_percent);

    let processing = state.router_status == "PROCESSING";
    let (ollama_indicator, ollama_color) = get_status_indicator(state.router_online, processing);

    let metrics_text = vec![
        Line::from(vec![
            Span::raw("CPU "),
            Span::styled(cpu_bar, Style::default().fg(cpu_color)),
        ]),
        Line::from(vec![
            Span::raw("RAM "),
            Span::styled(ram_bar, Style::default().fg(ram_color)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled(ollama_indicator, Style::default().fg(ollama_color)),
            Span::raw(format!(" Ollama {}", state.router_status)),
        ]),
        Line::from(""),
        Line::from(vec![
            Span::styled("○", Style::default().fg(Color::DarkGray)),
            Span::raw(" NVIDIA Coming Soon"),
        ]),
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
