// StartScreen.swift
// Start (Consent) screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Minimal consent journey.
// CRITICAL: No real OAuth.

import SwiftUI

struct StartScreen: View {
    @Binding var navigateTo: AppDestination?
    @State private var consented = false

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Today") {
                navigateTo = .today
            }

            // Header
            PageHeader(
                title: "Start",
                subtitle: "Connect sources so we can watch quietly."
            )

            // Consent explanation
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                Text("What we do:")
                    .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space2) {
                    QLBullet("Read from connected sources")
                    QLBullet("Look for patterns and obligations")
                    QLBullet("Hold things quietly until they matter")
                    QLBullet("Choose not to interrupt you")
                }

                Text("What we don't do:")
                    .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                    .padding(.top, DesignTokens.Spacing.space4)

                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space2) {
                    QLBullet("Store your messages or content")
                    QLBullet("Share data with anyone")
                    QLBullet("Send notifications unless critical")
                    QLBullet("Make decisions for you")
                }
            }
            .padding(.top, DesignTokens.Spacing.space6)

            // Consent action
            VStack(spacing: DesignTokens.Spacing.space4) {
                if !consented {
                    QLButton("I understand", style: .primary) {
                        consented = true
                    }
                } else {
                    WhisperText("Thank you.")

                    QLButton("Connect sources", style: .secondary) {
                        navigateTo = .connections
                    }
                    .padding(.top, DesignTokens.Spacing.space4)
                }
            }
            .padding(.top, DesignTokens.Spacing.space8)
        }
    }
}
