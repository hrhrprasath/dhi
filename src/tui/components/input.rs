use crate::tui::palette::Palette;
use crate::AppState;
use ratatui::{
    layout::Rect,
    style::Style,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let input = Paragraph::new(format!("> {}", state.input_buffer))
        .style(Style::default().fg(Palette::WARNING))
        .block(Block::default().borders(Borders::ALL).title(" > "));
    f.render_widget(input, area);
}
