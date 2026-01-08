// DeviceRegistrationStore.swift
// Device registration state management
//
// Phase 37: Device Registration + Deep-Link Receipt Landing
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
//
// CRITICAL: Registration is EXPLICIT (user-initiated).
// CRITICAL: Raw token sent to server, which seals it.
// CRITICAL: Only token hash stored locally.

import Foundation
import SwiftUI
import CryptoKit
import UserNotifications

/// State of device registration.
enum DeviceRegState: String {
    case notRegistered
    case requestingPermission
    case awaitingToken
    case registering
    case registered
    case failed
}

/// Store for device registration state.
/// CRITICAL: Raw token sent to server once, then discarded.
/// CRITICAL: Only hash stored locally.
class DeviceRegistrationStore: ObservableObject {
    static let shared = DeviceRegistrationStore()

    @Published private(set) var state: DeviceRegState = .notRegistered
    @Published private(set) var errorMessage: String?
    @Published private(set) var tokenHashPrefix: String?

    // Server configuration (mock for now)
    private let serverBaseURL = "http://localhost:8080"

    // Local circle ID (deterministic for this device)
    private var circleID: String {
        // Use device identifier hash as circle ID
        let vendorID = UIDevice.current.identifierForVendor?.uuidString ?? "unknown"
        return hashString(vendorID)
    }

    private init() {
        // Check if already registered
        if let storedHash = UserDefaults.standard.string(forKey: "ql_device_token_hash") {
            state = .registered
            tokenHashPrefix = String(storedHash.prefix(8))
        }
    }

    // MARK: - Public API

    /// Request push notification permission and register device.
    /// CRITICAL: User-initiated. No auto-register.
    func requestRegistration() {
        guard state != .registered && state != .registering else { return }

        state = .requestingPermission
        errorMessage = nil

        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            DispatchQueue.main.async {
                if let error = error {
                    self.state = .failed
                    self.errorMessage = error.localizedDescription
                    return
                }

                if granted {
                    self.state = .awaitingToken
                    UIApplication.shared.registerForRemoteNotifications()
                } else {
                    self.state = .failed
                    self.errorMessage = "Permission denied"
                }
            }
        }
    }

    /// Called when APNs token is received.
    /// CRITICAL: Token sent to server, then discarded locally.
    func receivedToken(_ token: String) {
        guard state == .awaitingToken else { return }

        state = .registering

        // Compute token hash for local storage
        let tokenHash = hashString(token)

        // Send to server
        registerWithServer(token: token, tokenHash: tokenHash)
    }

    /// Called when registration fails.
    func registrationFailed(error: String) {
        DispatchQueue.main.async {
            self.state = .failed
            self.errorMessage = error
        }
    }

    // MARK: - Private

    private func registerWithServer(token: String, tokenHash: String) {
        guard let url = URL(string: "\(serverBaseURL)/devices/register") else {
            registrationFailed(error: "Invalid server URL")
            return
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")

        // Form data
        let formData = [
            "circle_id": circleID,
            "platform": "ios",
            "device_token": token,
            "bundle_id": Bundle.main.bundleIdentifier ?? ""
        ]
        request.httpBody = formData.map { "\($0.key)=\($0.value)" }.joined(separator: "&").data(using: .utf8)

        URLSession.shared.dataTask(with: request) { data, response, error in
            DispatchQueue.main.async {
                if let error = error {
                    self.state = .failed
                    self.errorMessage = error.localizedDescription
                    return
                }

                guard let httpResponse = response as? HTTPURLResponse else {
                    self.state = .failed
                    self.errorMessage = "Invalid response"
                    return
                }

                if httpResponse.statusCode == 200 || httpResponse.statusCode == 302 {
                    // Success - store hash locally
                    UserDefaults.standard.set(tokenHash, forKey: "ql_device_token_hash")
                    self.state = .registered
                    self.tokenHashPrefix = String(tokenHash.prefix(8))
                    self.errorMessage = nil
                } else {
                    self.state = .failed
                    self.errorMessage = "Server error: \(httpResponse.statusCode)"
                }
            }
        }.resume()
    }

    /// Compute SHA256 hash of a string.
    private func hashString(_ input: String) -> String {
        let data = Data(input.utf8)
        let hash = SHA256.hash(data: data)
        return hash.compactMap { String(format: "%02x", $0) }.joined()
    }
}
