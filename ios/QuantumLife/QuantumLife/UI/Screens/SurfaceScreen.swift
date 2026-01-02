// SurfaceScreen.swift
// Something you could look at (Surface) screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Explain why surfaced, never what.
// CRITICAL: Hold action records hash only.

import SwiftUI

struct SurfaceScreen: View {
    @Binding var navigateTo: AppDestination?
    @State private var showExplain = false
    @State private var held = false

    private let model = SeededDataGenerator.shared.generateSurfaceItem()

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Today") {
                navigateTo = .today
            }

            if let item = model {
                // Header
                PageHeader(
                    title: "Something you could look at.",
                    subtitle: item.reasonSummary
                )

                // Category and horizon
                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                    HStack(spacing: DesignTokens.Spacing.space2) {
                        QLChip(item.category.displayText)
                        QLChip(item.horizon.displayText)
                    }

                    // Magnitude
                    Text("This represents \(item.magnitude.displayText) items.")
                        .font(.system(size: DesignTokens.Typography.textSM))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                }
                .padding(.top, DesignTokens.Spacing.space4)

                // Explain section (expandable)
                if showExplain {
                    QLCard {
                        VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
                            Text("Why this surfaced:")
                                .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                            ForEach(item.explainLines, id: \.self) { line in
                                QLBullet(line)
                            }
                        }
                    }
                    .padding(.top, DesignTokens.Spacing.space4)
                }

                // Actions
                VStack(spacing: DesignTokens.Spacing.space4) {
                    if !showExplain {
                        WhisperLink("Explain") {
                            withAnimation {
                                showExplain = true
                            }
                        }
                    }

                    if !held {
                        QLButton("Hold this for me", style: .secondary) {
                            HoldActionStore.shared.recordHold(itemKeyHash: item.itemKeyHash)
                            held = true
                        }
                    } else {
                        WhisperText("Held. We'll keep watching.")
                    }
                }
                .padding(.top, DesignTokens.Spacing.space8)

            } else {
                // Nothing to surface
                PageHeader(
                    title: "Nothing to surface.",
                    subtitle: "Everything that could need you has been considered. Nothing does."
                )
            }

            Spacer()
                .frame(height: DesignTokens.Spacing.space12)

            // Navigation whisper
            WhisperLink("Back to today") {
                navigateTo = .today
            }
        }
    }
}
