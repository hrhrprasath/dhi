pub mod components;
pub mod palette;

use crate::AppState;
use ratatui::Frame;

pub fn render(f: &mut Frame, state: &AppState) {
    components::render(f, state);
}
