// QuantumLifeApp.swift
// Main app entry point for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Navigation via state binding, not NavigationStack.
// CRITICAL: No deep linking or external routing.

import SwiftUI

@main
struct QuantumLifeApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}

// MARK: - App Destination

/// Navigation destinations within the app.
enum AppDestination: Hashable {
    case landing
    case today
    case held
    case surface
    case proof
    case start
    case connections
    case mirror
}

// MARK: - Content View

/// Root content view with navigation management.
struct ContentView: View {
    @State private var currentDestination: AppDestination? = .landing

    var body: some View {
        ZStack {
            DesignTokens.Colors.adaptiveBg
                .ignoresSafeArea()

            Group {
                switch currentDestination {
                case .landing, .none:
                    LandingScreen()
                        .onTapGesture {
                            // For demo: tap landing to proceed to today
                        }
                        .overlay(alignment: .bottom) {
                            // Demo: auto-advance to today after viewing landing
                            WhisperLink("Enter") {
                                withAnimation {
                                    currentDestination = .today
                                }
                            }
                            .padding(.bottom, DesignTokens.Spacing.space16)
                        }

                case .today:
                    TodayScreen(navigateTo: $currentDestination)

                case .held:
                    HeldScreen(navigateTo: $currentDestination)

                case .surface:
                    SurfaceScreen(navigateTo: $currentDestination)

                case .proof:
                    ProofScreen(navigateTo: $currentDestination)

                case .start:
                    StartScreen(navigateTo: $currentDestination)

                case .connections:
                    ConnectionsScreen(navigateTo: $currentDestination)

                case .mirror:
                    MirrorScreen(navigateTo: $currentDestination)
                }
            }
            .animation(.easeInOut(duration: DesignTokens.Motion.durationFast / 1000), value: currentDestination)
        }
    }
}

// MARK: - Preview

#Preview {
    ContentView()
}
