// ProofScreen.swift
// Quiet, kept. (Proof) screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Show restraint, not volume.
// CRITICAL: Record acknowledgement hash only.

import SwiftUI

struct ProofScreen: View {
    @Binding var navigateTo: AppDestination?
    @State private var acknowledged = false

    private let model = SeededDataGenerator.shared.generateProofSummary()

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Today") {
                navigateTo = .today
            }

            // Header
            PageHeader(
                title: "Quiet, kept.",
                subtitle: model.statement
            )

            // Categories where restraint was shown
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                Text("Categories where we chose not to interrupt:")
                    .font(.system(size: DesignTokens.Typography.textSM))
                    .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                HStack(spacing: DesignTokens.Spacing.space2) {
                    ForEach(model.categories, id: \.self) { category in
                        QLChip(category.displayText)
                    }
                }
            }
            .padding(.top, DesignTokens.Spacing.space6)

            // Why line
            QLCard {
                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space2) {
                    Text("Why we didn't tell you:")
                        .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                    Text(model.whyLine)
                        .font(.system(size: DesignTokens.Typography.textSM))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                }
            }
            .padding(.top, DesignTokens.Spacing.space4)

            // Acknowledge
            VStack(spacing: DesignTokens.Spacing.space4) {
                if !acknowledged {
                    WhisperLink("I saw this") {
                        AckStore.proof.record(hash: model.hash256())
                        acknowledged = true
                    }
                } else {
                    WhisperText("Acknowledged.")
                }
            }
            .padding(.top, DesignTokens.Spacing.space8)

            Spacer()
                .frame(height: DesignTokens.Spacing.space12)

            // Navigation whisper
            WhisperLink("Back to today") {
                navigateTo = .today
            }
        }
    }
}
