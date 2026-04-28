use crate::AppState;
use ratatui::{
    layout::Rect,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, _state: &AppState, area: Rect) {
    let command_center = Paragraph::new("Command center".to_string()).block(
        Block::default()
            .borders(Borders::ALL)
            .title(" COMMAND CENTER "),
    );
    f.render_widget(command_center, area);
}
