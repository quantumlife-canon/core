// HeldScreen.swift
// Held, quietly screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Only magnitude buckets, never counts.
// CRITICAL: Only category names, no specifics.

import SwiftUI

struct HeldScreen: View {
    @Binding var navigateTo: AppDestination?

    private let model = SeededDataGenerator.shared.generateHeldSummary()

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Today") {
                navigateTo = .today
            }

            // Header
            PageHeader(
                title: "Held, quietly.",
                subtitle: model.statement
            )

            // Categories being held
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                Text("Categories being watched:")
                    .font(.system(size: DesignTokens.Typography.textSM))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                HStack(spacing: DesignTokens.Spacing.space2) {
                    ForEach(model.categories, id: \.self) { category in
                        QLChip(category.displayText)
                    }
                }

                // Magnitude indicator
                Text("\(model.magnitude.displayText.capitalized) things are being held.")
                    .font(.system(size: DesignTokens.Typography.textSM))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
            }
            .padding(.top, DesignTokens.Spacing.space6)

            Spacer()
                .frame(height: DesignTokens.Spacing.space12)

            // Navigation whisper
            WhisperLink("Something you could look at") {
                navigateTo = .surface
            }
        }
    }
}
