use crate::services::logger::{LogEntry, LogLevel};
use crate::AppState;
use ratatui::{
    layout::Rect,
    style::{Color, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Scrollbar, ScrollbarOrientation, ScrollbarState},
    Frame,
};

pub struct LogState {
    pub scroll: u16,
    pub logs: Vec<LogEntry>,
}

impl LogState {
    pub fn new() -> Self {
        Self {
            scroll: 0,
            logs: Vec::new(),
        }
    }

    pub fn add_log(&mut self, entry: LogEntry) {
        self.logs.push(entry);
    }
}

impl Default for LogState {
    fn default() -> Self {
        Self::new()
    }
}

fn format_log_entry(entry: &LogEntry) -> Line<'static> {
    let level_style = match entry.level {
        LogLevel::Debug => Style::default(),
        LogLevel::Info => Style::default(),
        LogLevel::Warn => Style::default().fg(Color::Yellow),
        LogLevel::Error => Style::default().fg(Color::Red),
    };

    let component_style = Style::default().fg(Color::Cyan);
    let msg_style = Style::default().fg(Color::Gray);

    Line::from(vec![
        Span::raw("["),
        Span::styled(entry.component.clone(), component_style),
        Span::raw("]["),
        Span::styled(entry.level.as_str(), level_style),
        Span::raw("] "),
        Span::styled(entry.message.clone(), msg_style),
    ])
}

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let log_state = state.log_state.lock().unwrap();

    if log_state.logs.is_empty() {
        let empty = Paragraph::new("No logs yet...")
            .block(Block::default().borders(Borders::ALL).title(" LOGS "));
        f.render_widget(empty, area);
        return;
    }

    let _visible_lines = area.height as usize - 2;
    let content_len = log_state.logs.len();
    let current_scroll = log_state.scroll as usize;

    let lines: Vec<Line> = log_state.logs.iter().map(format_log_entry).collect();

    let log_view = Paragraph::new(lines)
        .block(Block::default().borders(Borders::ALL).title(" LOGS "))
        .scroll((log_state.scroll, 0));

    f.render_widget(log_view, area);

    let mut scroll_state = ScrollbarState::new(current_scroll).content_length(content_len);
    let scrollbar = Scrollbar::new(ScrollbarOrientation::VerticalRight)
        .track_symbol(Some(" "))
        .style(Style::default().fg(Color::DarkGray));
    f.render_stateful_widget(scrollbar, area, &mut scroll_state);
}

#[allow(dead_code)]
pub fn handle_scroll(state: &AppState, delta: i32) {
    let mut log_state = state.log_state.lock().unwrap();
    let visible_lines = 10;
    let max_scroll = log_state.logs.len().saturating_sub(visible_lines) as i32;

    let new_scroll = (log_state.scroll as i32 + delta).clamp(0, max_scroll);
    log_state.scroll = new_scroll as u16;
}
