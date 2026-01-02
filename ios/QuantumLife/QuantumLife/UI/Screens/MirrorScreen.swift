// MirrorScreen.swift
// Seen, quietly. (Mirror) screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Show restraint, not volume.
// CRITICAL: Abstract categories only.
// CRITICAL: Record acknowledgement hash.

import SwiftUI

struct MirrorScreen: View {
    @Binding var navigateTo: AppDestination?
    @State private var acknowledged = false

    private let connectionStore = ConnectionIntentStore.shared
    private var model: MirrorPageModel {
        let connections = connectionStore.getAllStates()
        return SeededDataGenerator.shared.generateMirrorPage(connections: connections)
    }

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Connections") {
                navigateTo = .connections
            }

            // Header
            PageHeader(
                title: model.title,
                subtitle: model.subtitle
            )

            // Sources
            if !model.sources.isEmpty {
                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                    Text("What we read:")
                        .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                    ForEach(model.sources, id: \.kind) { source in
                        MirrorSourceView(source: source)
                    }
                }
                .padding(.top, DesignTokens.Spacing.space6)

                // Outcome
                QLCard {
                    VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
                        if model.outcome.heldQuietly {
                            Text("We held \(model.outcome.heldMagnitude.displayText) things quietly.")
                                .font(.system(size: DesignTokens.Typography.textSM))
                                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                        }

                        if model.outcome.nothingRequiresAttention {
                            Text("Nothing requires your attention.")
                                .font(.system(size: DesignTokens.Typography.textSM))
                                .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                        }
                    }
                }
                .padding(.top, DesignTokens.Spacing.space4)

                // Restraint statement
                VStack(alignment: .center, spacing: DesignTokens.Spacing.space2) {
                    Text(model.restraintStatement)
                        .font(.system(size: DesignTokens.Typography.textSM))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                    WhisperText(model.restraintWhy)
                }
                .frame(maxWidth: .infinity)
                .padding(.top, DesignTokens.Spacing.space6)

            } else {
                // No connected sources
                VStack(alignment: .center, spacing: DesignTokens.Spacing.space4) {
                    Text("No sources connected yet.")
                        .font(.system(size: DesignTokens.Typography.textSM))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                    QLButton("Connect sources", style: .secondary) {
                        navigateTo = .connections
                    }
                }
                .frame(maxWidth: .infinity)
                .padding(.top, DesignTokens.Spacing.space8)
            }

            // Acknowledge
            if !model.sources.isEmpty {
                VStack(spacing: DesignTokens.Spacing.space4) {
                    if !acknowledged {
                        WhisperLink("I saw this") {
                            AckStore.mirror.record(hash: model.hash256())
                            acknowledged = true
                        }
                    } else {
                        WhisperText("Acknowledged.")
                    }
                }
                .padding(.top, DesignTokens.Spacing.space8)
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

// MARK: - Mirror Source View

struct MirrorSourceView: View {
    let source: MirrorSourceSummary

    var body: some View {
        QLCard {
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
                // Source header
                HStack {
                    Text(source.kind.displayText)
                        .font(.system(size: DesignTokens.Typography.textBase, weight: DesignTokens.Typography.fontMedium))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)

                    Spacer()

                    if source.readSuccessfully {
                        Text("Read")
                            .font(.system(size: DesignTokens.Typography.textXS))
                            .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                    }
                }

                // Not stored
                if !source.notStored.isEmpty {
                    Text("Not stored: \(source.notStored.joined(separator: ", "))")
                        .font(.system(size: DesignTokens.Typography.textXS))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextQuaternary)
                }

                // Observed items
                if !source.observed.isEmpty {
                    VStack(alignment: .leading, spacing: DesignTokens.Spacing.space1) {
                        ForEach(source.observed, id: \.category) { item in
                            HStack(spacing: DesignTokens.Spacing.space2) {
                                QLChip(item.category.displayText)
                                WhisperText("\(item.magnitude.displayText) • \(item.horizon.displayText)")
                            }
                        }
                    }
                }
            }
        }
    }
}
