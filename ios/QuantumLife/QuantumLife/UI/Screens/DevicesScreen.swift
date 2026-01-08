// DevicesScreen.swift
// Device registration screen
//
// Phase 37: Device Registration + Deep-Link Receipt Landing
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
//
// CRITICAL: Registration is EXPLICIT (user presses button).
// CRITICAL: A registered device does NOT mean interrupts are enabled.

import SwiftUI

struct DevicesScreen: View {
    @Binding var navigateTo: AppDestination?
    @ObservedObject private var store = DeviceRegistrationStore.shared

    var body: some View {
        PageContainer {
            // Back link
            BackLink("Connections") {
                navigateTo = .connections
            }

            // Header
            PageHeader(
                title: "Device, quietly.",
                subtitle: "Register this device to receive notifications."
            )

            // Registration status card
            VStack(spacing: DesignTokens.Spacing.space6) {
                QLCard {
                    VStack(alignment: .leading, spacing: DesignTokens.Spacing.space4) {
                        HStack {
                            Text("iOS Device")
                                .font(.system(size: DesignTokens.Typography.textBase, weight: DesignTokens.Typography.fontMedium))
                                .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)

                            Spacer()

                            statusBadge
                        }

                        Text(statusDescription)
                            .font(.system(size: DesignTokens.Typography.textSM))
                            .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                        if let hashPrefix = store.tokenHashPrefix {
                            Text("Token: \(hashPrefix)...")
                                .font(.system(size: DesignTokens.Typography.textXS, design: .monospaced))
                                .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                        }

                        if let error = store.errorMessage {
                            Text(error)
                                .font(.system(size: DesignTokens.Typography.textXS))
                                .foregroundColor(DesignTokens.Colors.errorRed)
                        }

                        if store.state != .registered {
                            QLButton(buttonTitle, style: .primary) {
                                store.requestRegistration()
                            }
                            .disabled(store.state == .registering || store.state == .awaitingToken)
                            .padding(.top, DesignTokens.Spacing.space2)
                        }
                    }
                }

                // Explanation card
                QLCard {
                    VStack(alignment: .leading, spacing: DesignTokens.Spacing.space2) {
                        Text("What this means")
                            .font(.system(size: DesignTokens.Typography.textSM, weight: DesignTokens.Typography.fontMedium))
                            .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)

                        Text("Registering allows us to send calm notifications when something needs your attention. The device token is sealed and encrypted — we can only use it to notify, not to identify.")
                            .font(.system(size: DesignTokens.Typography.textSM))
                            .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)

                        Text("Registration does not enable interrupts. You still control when we can notify you.")
                            .font(.system(size: DesignTokens.Typography.textSM))
                            .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
                            .padding(.top, DesignTokens.Spacing.space2)
                    }
                }
            }
            .padding(.top, DesignTokens.Spacing.space6)

            // Navigation whisper
            VStack(spacing: DesignTokens.Spacing.space4) {
                WhisperLink("See proof") {
                    // Would navigate to /proof/device on web
                    navigateTo = .today
                }

                WhisperLink("Back to today") {
                    navigateTo = .today
                }
            }
            .padding(.top, DesignTokens.Spacing.space12)
        }
    }

    // MARK: - Computed Properties

    private var statusBadge: some View {
        HStack(spacing: DesignTokens.Spacing.space1) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)

            Text(statusText)
                .font(.system(size: DesignTokens.Typography.textXS))
                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
        }
    }

    private var statusColor: Color {
        switch store.state {
        case .registered:
            return DesignTokens.Colors.successGreen
        case .failed:
            return DesignTokens.Colors.errorRed
        case .registering, .awaitingToken, .requestingPermission:
            return DesignTokens.Colors.warnYellow
        case .notRegistered:
            return DesignTokens.Colors.adaptiveTextTertiary
        }
    }

    private var statusText: String {
        switch store.state {
        case .registered:
            return "Registered"
        case .failed:
            return "Failed"
        case .registering:
            return "Registering..."
        case .awaitingToken:
            return "Awaiting token..."
        case .requestingPermission:
            return "Requesting..."
        case .notRegistered:
            return "Not registered"
        }
    }

    private var statusDescription: String {
        switch store.state {
        case .registered:
            return "This device can receive notifications when you permit it."
        case .failed:
            return "Registration failed. Try again when ready."
        case .registering, .awaitingToken, .requestingPermission:
            return "Setting up notifications..."
        case .notRegistered:
            return "Register to receive calm notifications."
        }
    }

    private var buttonTitle: String {
        switch store.state {
        case .failed:
            return "Try Again"
        case .registering, .awaitingToken, .requestingPermission:
            return "Registering..."
        default:
            return "Register this device"
        }
    }
}

// MARK: - Preview

#Preview {
    DevicesScreen(navigateTo: .constant(nil))
}
