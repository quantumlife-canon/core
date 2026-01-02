// TodayScreen.swift
// Today, quietly screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: No specific details, only abstract observations.
// CRITICAL: Single whisper cue at most.

import SwiftUI

struct TodayScreen: View {
    @Binding var navigateTo: AppDestination?

    private let preferenceStore = PreferenceStore.shared
    private var model: TodayPageModel {
        SeededDataGenerator.shared.generateTodayPage(preference: preferenceStore.currentMode)
    }

    var body: some View {
        PageContainer {
            // Header
            PageHeader(
                title: model.greeting,
                subtitle: nil
            )

            // Observations
            VStack(alignment: .leading, spacing: DesignTokens.Spacing.space3) {
                ForEach(model.observations, id: \.self) { observation in
                    QLBullet(observation)
                }
            }

            // Whisper cue (if present)
            if let cue = model.whisperCue {
                WhisperCueView(cue: cue, navigateTo: $navigateTo)
                    .padding(.top, DesignTokens.Spacing.space6)
            }

            Spacer()
                .frame(height: DesignTokens.Spacing.space12)

            // Navigation whispers
            VStack(spacing: DesignTokens.Spacing.space4) {
                WhisperLink("What we're holding") {
                    navigateTo = .held
                }

                WhisperLink("Proof of quiet") {
                    navigateTo = .proof
                }

                WhisperLink("Start") {
                    navigateTo = .start
                }
            }
        }
    }
}

// MARK: - Whisper Cue View

struct WhisperCueView: View {
    let cue: WhisperCue
    @Binding var navigateTo: AppDestination?

    var body: some View {
        VStack(alignment: .center, spacing: DesignTokens.Spacing.space3) {
            WhisperText(cue.text)
                .multilineTextAlignment(.center)

            WhisperLink(cue.linkText) {
                switch cue.type {
                case .surface:
                    navigateTo = .surface
                case .proof:
                    navigateTo = .proof
                }
            }
        }
        .frame(maxWidth: .infinity)
    }
}
