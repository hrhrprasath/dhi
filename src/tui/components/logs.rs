use crate::AppState;
use ratatui::{
    layout::Rect,
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let logs: Vec<Line> = state.logs.iter().map(|l| Line::from(l.as_str())).collect();
    let log_view =
        Paragraph::new(logs).block(Block::default().borders(Borders::ALL).title(" LOG "));
    f.render_widget(log_view, area);
}
