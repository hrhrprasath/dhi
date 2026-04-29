use ratatui::style::Color;

pub struct Palette;

impl Palette {
    pub const TEXT: Color = Color::White;
    pub const TEXT_DIM: Color = Color::Gray;
    pub const TEXT_ACCENT: Color = Color::Cyan;

    pub const BORDER_FOCUSED: Color = Color::Rgb(255, 165, 0);
    pub const BORDER_UNFOCUSED: Color = Color::DarkGray;

    pub const ACCENT: Color = Color::Rgb(255, 165, 0);

    pub const SUCCESS: Color = Color::Green;
    pub const WARNING: Color = Color::Yellow;
    pub const ERROR: Color = Color::Red;

    pub const INACTIVE: Color = Color::DarkGray;
    pub const LABEL: Color = Color::Blue;
}
