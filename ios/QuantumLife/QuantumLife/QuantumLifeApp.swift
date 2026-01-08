// QuantumLifeApp.swift
// Main app entry point for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Phase 37: Device Registration + Deep-Link Receipt Landing
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
//
// CRITICAL: Navigation via state binding, not NavigationStack.
// CRITICAL: Deep link URL scheme: quantumlife://open?t=...
// CRITICAL: No identifiers in deep links.

import SwiftUI
import UserNotifications

@main
struct QuantumLifeApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            ContentView()
                .onOpenURL { url in
                    handleDeepLink(url: url)
                }
        }
    }

    /// Handle incoming deep links.
    /// URL scheme: quantumlife://open?t=interrupts|trust|shadow|reality|today
    /// CRITICAL: No identifiers in URLs.
    private func handleDeepLink(url: URL) {
        guard url.scheme == "quantumlife",
              url.host == "open",
              let components = URLComponents(url: url, resolvingAgainstBaseURL: true),
              let queryItems = components.queryItems,
              let tValue = queryItems.first(where: { $0.name == "t" })?.value else {
            return
        }

        // Validate t parameter
        guard let target = DeepLinkTarget(rawValue: tValue) else {
            return
        }

        // Post notification for ContentView to handle
        NotificationCenter.default.post(
            name: .deepLinkReceived,
            object: nil,
            userInfo: ["target": target]
        )
    }
}

// MARK: - App Delegate

/// Minimal app delegate for push notification registration.
/// CRITICAL: Registration is EXPLICIT (user-initiated).
class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey : Any]? = nil) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        return true
    }

    /// Called when APNs device token is received.
    /// CRITICAL: Token is passed to DeviceRegistrationStore for server registration.
    func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        let tokenString = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
        DeviceRegistrationStore.shared.receivedToken(tokenString)
    }

    func application(_ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
        DeviceRegistrationStore.shared.registrationFailed(error: error.localizedDescription)
    }

    // MARK: - UNUserNotificationCenterDelegate

    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .sound])
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, completionHandler: @escaping () -> Void) {
        // Handle notification tap - extract deep link target
        if let urlString = response.notification.request.content.userInfo["url"] as? String,
           let url = URL(string: urlString) {
            UIApplication.shared.open(url)
        }
        completionHandler()
    }
}

// MARK: - Deep Link Target

/// Valid deep link targets.
/// CRITICAL: No identifiers. Only abstract screen names.
enum DeepLinkTarget: String, CaseIterable {
    case interrupts
    case trust
    case shadow
    case reality
    case today

    var destination: AppDestination {
        switch self {
        case .interrupts: return .interruptPreview
        case .trust: return .trustReceipt
        case .shadow: return .shadowReceipt
        case .reality: return .reality
        case .today: return .today
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
    // Phase 37: Deep link targets
    case interruptPreview
    case trustReceipt
    case shadowReceipt
    case reality
    case devices
}

// MARK: - Notification Names

extension Notification.Name {
    static let deepLinkReceived = Notification.Name("deepLinkReceived")
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

                // Phase 37: Deep link screens
                case .interruptPreview:
                    PlaceholderScreen(
                        title: "Interrupt Preview",
                        message: "Noted. Open on web for now.",
                        navigateTo: $currentDestination
                    )

                case .trustReceipt:
                    PlaceholderScreen(
                        title: "Trust Receipt",
                        message: "Noted. Open on web for now.",
                        navigateTo: $currentDestination
                    )

                case .shadowReceipt:
                    PlaceholderScreen(
                        title: "Shadow Receipt",
                        message: "Noted. Open on web for now.",
                        navigateTo: $currentDestination
                    )

                case .reality:
                    PlaceholderScreen(
                        title: "Reality",
                        message: "Noted. Open on web for now.",
                        navigateTo: $currentDestination
                    )

                case .devices:
                    DevicesScreen(navigateTo: $currentDestination)
                }
            }
            .animation(.easeInOut(duration: DesignTokens.Motion.durationFast / 1000), value: currentDestination)
        }
        .onReceive(NotificationCenter.default.publisher(for: .deepLinkReceived)) { notification in
            if let target = notification.userInfo?["target"] as? DeepLinkTarget {
                withAnimation {
                    currentDestination = target.destination
                }
            }
        }
    }
}

// MARK: - Placeholder Screen

/// Minimal placeholder for deep link targets not yet implemented.
struct PlaceholderScreen: View {
    let title: String
    let message: String
    @Binding var navigateTo: AppDestination?

    var body: some View {
        PageContainer {
            BackLink("Today") {
                navigateTo = .today
            }

            PageHeader(
                title: title,
                subtitle: nil
            )

            Text(message)
                .font(.system(size: DesignTokens.Typography.textBase))
                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                .padding(.top, DesignTokens.Spacing.space8)
        }
    }
}

// MARK: - Preview

#Preview {
    ContentView()
}
