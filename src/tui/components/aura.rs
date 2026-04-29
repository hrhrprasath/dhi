use crate::tui::palette::Palette;
use crate::AppState;
use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let aura_border = Block::default()
        .borders(Borders::ALL)
        .title(" AURA OS ")
        .title_style(Style::default().add_modifier(Modifier::BOLD));
    f.render_widget(aura_border, area);

    let aura_inner = Rect::new(area.x + 1, area.y + 1, area.right() - 1, area.bottom() - 1);

    let aura_split = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Length(14), Constraint::Min(10)])
        .split(aura_inner);

    let info_text = vec![
        Line::from("OS: Aura v1.0"),
        Line::from("Host: Mac M4"),
        Line::from("Kernel: "),
        Line::from("Up: 00:00:04"),
        Line::from("\u{2600}\u{fe0f} 72\u{00b0}F"),
    ];
    let info_widget = Paragraph::new(info_text).style(Style::default().fg(Palette::TEXT));
    f.render_widget(info_widget, aura_split[0]);

    let anim = state.animation_frame % 12;

    // let anim_text = match anim {
    //     // PHASE 1: THE INHALE (0-14) - Slow expansion from a single point
    //     0..=2 => "        \n        \n        \n    •   \n        \n        \n        \n        ",
    //     3..=5 => "        \n        \n        \n   ••   \n        \n        \n        \n        ",
    //     6..=8 => "        \n        \n    •   \n   ••   \n        \n        \n        \n        ",
    //     9..=11 => "        \n        \n    •   \n   •••  \n    •   \n        \n        \n        ",
    //     12..=14 => "        \n    •   \n   • •  \n    •   \n        \n        \n        \n        ",

    //     // PHASE 2: RADIATING (15-29) - Reaching out to the corners
    //     15..=17 => "    •   \n        \n  • • • \n    •   \n        \n        \n        \n        ",
    //     18..=20 => "    •   \n  •   • \n    •   \n  •   • \n    •   \n        \n        \n        ",
    //     21..=23 => "  •   • \n    •   \n •     •\n    •   \n •     •\n    •   \n  •   • \n        ",
    //     24..=26 => " • • • •\n  • • • \n   • •  \n    •   \n   • •  \n  • • • \n • • • •\n        ",
    //     27..=29 => "• • • • \n • • • •\n• • • • \n • • • •\n• • • • \n • • • •\n• • • • \n        ",

    //     // PHASE 3: ACTIVE SYNTHESIS (30-44) - The "Jiggle" / High energy
    //     30..=32 => " • • • •\n• • • • \n • • • •\n• • • • \n • • • •\n• • • • \n • • • •\n        ",
    //     33..=35 => "• • • • \n • • • •\n• • • • \n • • • •\n• • • • \n • • • •\n• • • • \n        ",
    //     36..=38 => "  • • • \n • • • •\n• • • • \n • • • •\n• • • • \n • • • •\n  • • • \n        ",
    //     39..=41 => "   • •  \n  • • • \n • • • •\n• • • • \n • • • •\n  • • • \n   • •  \n        ",
    //     42..=44 => "        \n   • •  \n  • • • \n • • • •\n  • • • \n   • •  \n        \n        ",

    //     // PHASE 4: THE EXHALE (45-59) - Softening and returning to core
    //     45..=47 => "        \n        \n   • •  \n  • • • \n   • •  \n        \n        \n        ",
    //     48..=50 => "        \n        \n    •   \n   • •  \n    •   \n        \n        \n        ",
    //     51..=53 => "        \n        \n        \n   • •  \n        \n        \n        \n        ",
    //     54..=56 => "        \n        \n        \n    ••  \n        \n        \n        \n        ",
    //     57..=59 => "        \n        \n        \n    •   \n        \n        \n        \n        ",

    //     _ => "        \n        \n        \n    •   \n        \n        \n        \n        ",
    // };

    let anim_text = match anim {
        // 0-1: The Core Seed
        0 => "        \n        \n        \n    •   \n        \n        \n        \n        ",
        1 => "        \n        \n        \n   ••   \n        \n        \n        \n        ",

        // 2-3: Vertical Expansion
        2 => "        \n        \n    •   \n   ••   \n    •   \n        \n        \n        ",
        3 => "        \n    •   \n        \n  • • • \n        \n    •   \n        \n        ",

        // 4-5: Peak "Aura" (Maximum Spread)
        4 => "    •   \n        \n •     •\n    •   \n •     •\n        \n    •   \n        ",
        5 => "  • • • \n •     •\n•  • •  \n • • • •\n•  • •  \n •     •\n  • • • \n        ",

        // 6-7: The "Exhale" (Contracting)
        6 => "        \n  • • • \n •     •\n •  •  •\n •     •\n  • • • \n        \n        ",
        7 => "        \n        \n   • •  \n   • •  \n   • •  \n        \n        \n        ",

        // 8-9: Return to Zero
        8 => "        \n        \n        \n   ••   \n   ••   \n        \n        \n        ",
        9 => "        \n        \n        \n    •   \n        \n        \n        \n        ",

        _ => "        \n        \n        \n    •   \n        \n        \n        \n        ",
    };

    let lines: Vec<Line> = anim_text
        .lines()
        .map(|l| {
            let spans: Vec<ratatui::text::Span> = l
                .chars()
                .map(|c| {
                    if c == '•' {
                        ratatui::text::Span::styled("•", Style::default().fg(Palette::ACCENT))
                    } else {
                        ratatui::text::Span::raw(" ")
                    }
                })
                .collect();
            Line::from(spans)
        })
        .collect();

    let anim_widget = Paragraph::new(lines);
    f.render_widget(anim_widget, aura_split[1]);
}
