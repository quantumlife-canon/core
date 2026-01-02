// Components.swift
// Reusable UI components for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Components must use DesignTokens exclusively.
// CRITICAL: No hardcoded values.

import SwiftUI

// MARK: - QLButton

/// Primary and secondary button styles matching web design.
struct QLButton: View {
    enum Style {
        case primary
        case secondary
    }

    let title: String
    let style: Style
    let action: () -> Void

    init(_ title: String, style: Style = .primary, action: @escaping () -> Void) {
        self.title = title
        self.style = style
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(.system(size: DesignTokens.Component.buttonFontSize, weight: DesignTokens.Typography.fontMedium))
                .foregroundColor(style == .primary ? DesignTokens.Colors.actionPrimaryText : DesignTokens.Colors.adaptiveTextPrimary)
                .padding(.horizontal, DesignTokens.Component.buttonPaddingX)
                .padding(.vertical, DesignTokens.Component.buttonPaddingY)
                .background(style == .primary ? DesignTokens.Colors.actionPrimary : Color.clear)
                .cornerRadius(DesignTokens.Component.buttonRadius)
                .overlay(
                    RoundedRectangle(cornerRadius: DesignTokens.Component.buttonRadius)
                        .stroke(style == .secondary ? DesignTokens.Colors.actionSecondaryBorder : Color.clear, lineWidth: 1)
                )
        }
    }
}

// MARK: - QLLink

/// Subtle link style matching web design.
struct QLLink: View {
    let text: String
    let action: () -> Void

    init(_ text: String, action: @escaping () -> Void) {
        self.text = text
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            Text(text)
                .font(.system(size: DesignTokens.Typography.textSM))
                .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                .underline(pattern: .dot, color: DesignTokens.Colors.adaptiveTextQuaternary)
        }
    }
}

// MARK: - WhisperText

/// Whisper-style text: tiny, low contrast, subtle.
struct WhisperText: View {
    let text: String

    init(_ text: String) {
        self.text = text
    }

    var body: some View {
        Text(text)
            .font(.system(size: DesignTokens.Typography.textXS))
            .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
    }
}

// MARK: - WhisperLink

/// Whisper-style link with dotted underline.
struct WhisperLink: View {
    let text: String
    let action: () -> Void

    init(_ text: String, action: @escaping () -> Void) {
        self.text = text
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            Text(text)
                .font(.system(size: DesignTokens.Typography.textXS))
                .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                .underline(pattern: .dot, color: DesignTokens.Colors.adaptiveTextQuaternary)
        }
    }
}

// MARK: - QLChip

/// Category chip for abstract display.
struct QLChip: View {
    let text: String

    init(_ text: String) {
        self.text = text
    }

    var body: some View {
        Text(text)
            .font(.system(size: DesignTokens.Typography.textXS))
            .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
            .padding(.horizontal, DesignTokens.Spacing.space2)
            .padding(.vertical, DesignTokens.Spacing.space1)
            .background(DesignTokens.Colors.adaptiveBorderSubtle)
            .cornerRadius(DesignTokens.Radius.sm)
    }
}

// MARK: - QLSection

/// Section container with optional header.
struct QLSection<Content: View>: View {
    let header: String?
    let content: Content

    init(header: String? = nil, @ViewBuilder content: () -> Content) {
        self.header = header
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
            if let header = header {
                Text(header)
                    .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
            }
            content
        }
    }
}

// MARK: - QLDivider

/// Subtle divider line.
struct QLDivider: View {
    var body: some View {
        Rectangle()
            .fill(DesignTokens.Colors.adaptiveBorderSubtle)
            .frame(height: 1)
    }
}

// MARK: - QLCard

/// Card container with subtle styling.
struct QLCard<Content: View>: View {
    let content: Content

    init(@ViewBuilder content: () -> Content) {
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
            content
        }
        .padding(DesignTokens.Component.cardPadding)
        .background(DesignTokens.Colors.adaptiveSurface)
        .cornerRadius(DesignTokens.Component.cardRadius)
    }
}

// MARK: - QLBullet

/// Bullet point for observations list.
struct QLBullet: View {
    let text: String

    init(_ text: String) {
        self.text = text
    }

    var body: some View {
        HStack(alignment: .top, spacing: DesignTokens.Spacing.space2) {
            Text("•")
                .foregroundColor(DesignTokens.Colors.adaptiveTextQuaternary)
            Text(text)
                .font(.system(size: DesignTokens.Typography.textSM))
                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
        }
    }
}

// MARK: - PageContainer

/// Standard page container with proper spacing.
struct PageContainer<Content: View>: View {
    let content: Content

    init(@ViewBuilder content: () -> Content) {
        self.content = content()
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space6) {
                content
            }
            .padding(DesignTokens.Component.containerPadding)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .background(DesignTokens.Colors.adaptiveBg)
    }
}

// MARK: - PageHeader

/// Standard page header with title and subtitle.
struct PageHeader: View {
    let title: String
    let subtitle: String?

    init(title: String, subtitle: String? = nil) {
        self.title = title
        self.subtitle = subtitle
    }

    var body: some View {
        VStack(alignment: .center, spacing: DesignTokens.Spacing.space2) {
            Text(title)
                .font(.system(size: DesignTokens.Typography.textXL, weight: DesignTokens.Typography.fontNormal))
                .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)

            if let subtitle = subtitle {
                Text(subtitle)
                    .font(.system(size: DesignTokens.Typography.textSM))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                    .multilineTextAlignment(.center)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, DesignTokens.Spacing.space8)
    }
}

// MARK: - BackLink

/// iOS-native back link with restrained styling.
struct BackLink: View {
    let text: String
    let action: () -> Void

    init(_ text: String = "Back", action: @escaping () -> Void) {
        self.text = text
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            HStack(spacing: DesignTokens.Spacing.space1) {
                Image(systemName: "chevron.left")
                    .font(.system(size: DesignTokens.Typography.textSM))
                Text(text)
                    .font(.system(size: DesignTokens.Typography.textSM))
            }
            .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
        }
    }
}
