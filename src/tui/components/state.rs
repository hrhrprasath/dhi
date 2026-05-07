use crate::AppState;
use ratatui::{
    layout::Rect,
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let state_text = vec![
        Line::from(format!("IN:  {} items", state.input_queue_count)),
        Line::from(format!("OUT: {} items", state.output_queue_count)),
    ];
    let state_block = Block::default().borders(Borders::ALL).title(" QUEUES ");
    let state_widget = Paragraph::new(state_text).block(state_block);
    f.render_widget(state_widget, area);
}
