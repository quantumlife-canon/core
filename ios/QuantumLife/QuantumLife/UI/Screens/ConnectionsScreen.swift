// ConnectionsScreen.swift
// Connections management screen
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Mock connections only.
// CRITICAL: Record connection intent hash only.

import SwiftUI

struct ConnectionsScreen: View {
    @Binding var navigateTo: AppDestination?

    private let connectionStore = ConnectionIntentStore.shared

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Start") {
                navigateTo = .start
            }

            // Header
            PageHeader(
                title: "Connections",
                subtitle: "Sources we can read from."
            )

            // Connection list
            VStack(spacing: DesignTokens.Spacing.space4) {
                ForEach(ConnectionKind.allCases, id: \.self) { kind in
                    ConnectionRow(
                        kind: kind,
                        status: connectionStore.getState(for: kind),
                        onConnect: {
                            connectionStore.recordIntent(kind: kind, action: "connect", mode: "mock")
                        },
                        onDisconnect: {
                            connectionStore.recordIntent(kind: kind, action: "disconnect", mode: "mock")
                        }
                    )
                }
            }
            .padding(.top, DesignTokens.Spacing.space6)

            // Navigation whisper
            VStack(spacing: DesignTokens.Spacing.space4) {
                WhisperLink("Manage device") {
                    navigateTo = .devices
                }

                WhisperLink("See what we've seen") {
                    navigateTo = .mirror
                }

                WhisperLink("Back to today") {
                    navigateTo = .today
                }
            }
            .padding(.top, DesignTokens.Spacing.space12)
        }
    }
}

// MARK: - Connection Row

struct ConnectionRow: View {
    let kind: ConnectionKind
    let status: ConnectionStatus
    let onConnect: () -> Void
    let onDisconnect: () -> Void

    @State private var currentStatus: ConnectionStatus

    init(kind: ConnectionKind, status: ConnectionStatus, onConnect: @escaping () -> Void, onDisconnect: @escaping () -> Void) {
        self.kind = kind
        self.status = status
        self.onConnect = onConnect
        self.onDisconnect = onDisconnect
        self._currentStatus = State(initialValue: status)
    }

    var body: some View {
        QLCard {
            HStack {
                VStack(alignment: .leading, spacing: DesignTokens.Spacing.space1) {
                    Text(kind.displayText)
                        .font(.system(size: DesignTokens.Typography.textBase, weight: DesignTokens.Typography.fontMedium))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)

                    Text(currentStatus.displayText)
                        .font(.system(size: DesignTokens.Typography.textXS))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                }

                Spacer()

                if currentStatus.isConnected {
                    QLButton("Disconnect", style: .secondary) {
                        onDisconnect()
                        currentStatus = .notConnected
                    }
                } else {
                    QLButton("Connect", style: .primary) {
                        onConnect()
                        currentStatus = .connectedMock
                    }
                }
            }
        }
    }
}
