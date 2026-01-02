// LocalStore.swift
// Hash-only local stores for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Only store hashes, never raw content.
// CRITICAL: No identifiers stored.
// CRITICAL: Minimal footprint.

import Foundation
import CryptoKit

// MARK: - Preference Store

/// Stores user preference as hash only.
final class PreferenceStore {
    private let defaults = UserDefaults.standard
    private let preferenceHashKey = "ql_preference_hash"
    private let preferenceModeKey = "ql_preference_mode"

    /// Current preference mode.
    var currentMode: PreferenceMode {
        get {
            guard let raw = defaults.string(forKey: preferenceModeKey),
                  let mode = PreferenceMode(rawValue: raw) else {
                return .quiet // Default
            }
            return mode
        }
        set {
            defaults.set(newValue.rawValue, forKey: preferenceModeKey)
            recordHash(for: newValue)
        }
    }

    /// Records a hash of the preference change (for audit trail).
    private func recordHash(for mode: PreferenceMode) {
        let timestamp = Date().timeIntervalSince1970
        let canonical = "PREF|\(mode.rawValue)|\(Int(timestamp))"
        let data = Data(canonical.utf8)
        let hash = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
        defaults.set(hash, forKey: preferenceHashKey)
    }

    /// Returns the last preference hash (for verification).
    var lastHash: String? {
        defaults.string(forKey: preferenceHashKey)
    }
}

// MARK: - Ack Store

/// Stores acknowledgement hashes for proof/mirror.
final class AckStore {
    private let defaults = UserDefaults.standard
    private let maxRecords = 128

    private var recordsKey: String { "ql_ack_records_\(storeType)" }
    private var indexKey: String { "ql_ack_index_\(storeType)" }

    enum StoreType: String {
        case proof = "proof"
        case mirror = "mirror"
    }

    let storeType: StoreType

    init(type: StoreType) {
        self.storeType = type
    }

    /// Records an acknowledgement hash.
    func record(hash: String) {
        var records = defaults.stringArray(forKey: recordsKey) ?? []
        var index = Set(defaults.stringArray(forKey: indexKey) ?? [])

        // Evict oldest if at capacity
        while records.count >= maxRecords {
            records.removeFirst()
        }

        // Append new record
        records.append(hash)
        index.insert(hash)

        defaults.set(records, forKey: recordsKey)
        defaults.set(Array(index), forKey: indexKey)
    }

    /// Checks if a hash has been acknowledged recently.
    func hasRecent(hash: String) -> Bool {
        let index = Set(defaults.stringArray(forKey: indexKey) ?? [])
        return index.contains(hash)
    }

    /// Returns the count of stored records.
    var count: Int {
        defaults.stringArray(forKey: recordsKey)?.count ?? 0
    }

    /// Clears all records (for testing).
    func clear() {
        defaults.removeObject(forKey: recordsKey)
        defaults.removeObject(forKey: indexKey)
    }
}

// MARK: - Hold Action Store

/// Stores hold action hashes.
final class HoldActionStore {
    private let defaults = UserDefaults.standard
    private let recordsKey = "ql_hold_records"
    private let maxRecords = 64

    /// Records a hold action.
    func recordHold(itemKeyHash: String) {
        let timestamp = Date().timeIntervalSince1970
        let canonical = "HOLD|\(itemKeyHash)|\(Int(timestamp))"
        let data = Data(canonical.utf8)
        let recordHash = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()

        var records = defaults.stringArray(forKey: recordsKey) ?? []

        // Evict oldest if at capacity
        while records.count >= maxRecords {
            records.removeFirst()
        }

        records.append(recordHash)
        defaults.set(records, forKey: recordsKey)
    }

    /// Returns the count of stored hold actions.
    var count: Int {
        defaults.stringArray(forKey: recordsKey)?.count ?? 0
    }

    /// Clears all records (for testing).
    func clear() {
        defaults.removeObject(forKey: recordsKey)
    }
}

// MARK: - Connection Intent Store

/// Stores connection intent hashes.
final class ConnectionIntentStore {
    private let defaults = UserDefaults.standard
    private let intentsKey = "ql_connection_intents"
    private let stateKey = "ql_connection_state"

    /// Records a connection intent.
    func recordIntent(kind: ConnectionKind, action: String, mode: String) {
        let timestamp = Date().timeIntervalSince1970
        let canonical = "CONN_INTENT|v1|\(kind.rawValue)|\(action)|\(mode)|\(Int(timestamp))"
        let data = Data(canonical.utf8)
        let hash = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()

        var intents = defaults.stringArray(forKey: intentsKey) ?? []
        intents.append(hash)
        defaults.set(intents, forKey: intentsKey)

        // Update state
        var state = defaults.dictionary(forKey: stateKey) as? [String: String] ?? [:]
        state[kind.rawValue] = action == "connect" ? "connected_mock" : "not_connected"
        defaults.set(state, forKey: stateKey)
    }

    /// Gets the current connection state.
    func getState(for kind: ConnectionKind) -> ConnectionStatus {
        let state = defaults.dictionary(forKey: stateKey) as? [String: String] ?? [:]
        guard let raw = state[kind.rawValue],
              let status = ConnectionStatus(rawValue: raw) else {
            return .notConnected
        }
        return status
    }

    /// Gets all connection states.
    func getAllStates() -> [ConnectionStateModel] {
        ConnectionKind.allCases.map { kind in
            ConnectionStateModel(kind: kind, status: getState(for: kind))
        }
    }

    /// Clears all records (for testing).
    func clear() {
        defaults.removeObject(forKey: intentsKey)
        defaults.removeObject(forKey: stateKey)
    }
}

// MARK: - Shared Instances

extension PreferenceStore {
    static let shared = PreferenceStore()
}

extension AckStore {
    static let proof = AckStore(type: .proof)
    static let mirror = AckStore(type: .mirror)
}

extension HoldActionStore {
    static let shared = HoldActionStore()
}

extension ConnectionIntentStore {
    static let shared = ConnectionIntentStore()
}
