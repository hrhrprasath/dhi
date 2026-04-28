use crate::AppState;
use ratatui::{
    layout::Rect,
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let state_text = vec![Line::from(state.system_state.clone())];
    let state_block = Block::default().borders(Borders::ALL).title(" STATE ");
    let state_widget = Paragraph::new(state_text).block(state_block);
    f.render_widget(state_widget, area);
}
