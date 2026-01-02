// Models.swift
// Domain models for QuantumLife iOS - mirrors server abstractions
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: No identifiers (names, emails, dates, amounts).
// CRITICAL: Use magnitude buckets only (none/a_few/several).
// CRITICAL: Use horizon buckets only (recent/ongoing/earlier).

import Foundation
import CryptoKit

// MARK: - Magnitude Bucket

/// Abstract magnitude bucket - never raw counts.
enum MagnitudeBucket: String, CaseIterable, Hashable {
    case none = "none"
    case aFew = "a_few"
    case several = "several"

    var displayText: String {
        switch self {
        case .none: return "nothing"
        case .aFew: return "a few"
        case .several: return "several"
        }
    }

    static func from(count: Int) -> MagnitudeBucket {
        switch count {
        case 0: return .none
        case 1...3: return .aFew
        default: return .several
        }
    }
}

// MARK: - Horizon Bucket

/// Abstract time horizon - never timestamps.
enum HorizonBucket: String, CaseIterable, Hashable {
    case recent = "recent"
    case ongoing = "ongoing"
    case earlier = "earlier"

    var displayText: String {
        switch self {
        case .recent: return "recently"
        case .ongoing: return "ongoing"
        case .earlier: return "earlier"
        }
    }
}

// MARK: - Category

/// Abstract life category - no specific identifiers.
enum Category: String, CaseIterable, Hashable {
    case money = "money"
    case time = "time"
    case work = "work"
    case people = "people"
    case home = "home"

    var displayText: String {
        rawValue.capitalized
    }
}

// MARK: - Connection Kind

/// Connection source type.
enum ConnectionKind: String, CaseIterable, Hashable {
    case email = "email"
    case calendar = "calendar"
    case finance = "finance"

    var displayText: String {
        rawValue.capitalized
    }
}

// MARK: - Connection Status

/// Connection state.
enum ConnectionStatus: String, Hashable {
    case notConnected = "not_connected"
    case connectedMock = "connected_mock"
    case connectedReal = "connected_real"
    case needsConfig = "needs_config"

    var displayText: String {
        switch self {
        case .notConnected: return "Not connected"
        case .connectedMock: return "Connected (mock)"
        case .connectedReal: return "Connected"
        case .needsConfig: return "Needs configuration"
        }
    }

    var isConnected: Bool {
        self == .connectedMock || self == .connectedReal
    }
}

// MARK: - Preference

/// User preference mode.
enum PreferenceMode: String, Hashable {
    case quiet = "quiet"
    case showAll = "show_all"
}

// MARK: - Observed Category

/// What kind of data was observed.
enum ObservedCategory: String, CaseIterable, Hashable {
    case timeCommitments = "time_commitments"
    case receipts = "receipts"
    case messages = "messages"
    case patterns = "patterns"

    var displayText: String {
        switch self {
        case .timeCommitments: return "time commitments"
        case .receipts: return "receipts"
        case .messages: return "messages"
        case .patterns: return "patterns"
        }
    }
}

// MARK: - Today Page Model

/// View model for the /today screen.
struct TodayPageModel: Hashable {
    let greeting: String
    let observations: [String]
    let whisperCue: WhisperCue?
    let preference: PreferenceMode

    func canonicalString() -> String {
        let obsJoined = observations.joined(separator: "|")
        let cueStr = whisperCue?.canonicalString() ?? "none"
        return "TODAY|v1|\(greeting)|\(obsJoined)|\(cueStr)|\(preference.rawValue)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Whisper Cue

/// Single whisper cue for /today.
struct WhisperCue: Hashable {
    enum CueType: String {
        case surface = "surface"
        case proof = "proof"
    }

    let type: CueType
    let text: String
    let linkText: String

    func canonicalString() -> String {
        "CUE|\(type.rawValue)|\(text)|\(linkText)"
    }
}

// MARK: - Held Summary Model

/// View model for the /held screen.
struct HeldSummaryModel: Hashable {
    let statement: String
    let categories: [Category]
    let magnitude: MagnitudeBucket

    func canonicalString() -> String {
        let cats = categories.map(\.rawValue).sorted().joined(separator: ",")
        return "HELD|v1|\(magnitude.rawValue)|\(cats)|\(statement)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Surface Item Model

/// View model for the /surface screen.
struct SurfaceItemModel: Hashable {
    let category: Category
    let magnitude: MagnitudeBucket
    let horizon: HorizonBucket
    let reasonSummary: String
    let explainLines: [String]
    let itemKeyHash: String

    func canonicalString() -> String {
        let lines = explainLines.joined(separator: "|")
        return "SURFACE|v1|\(category.rawValue)|\(magnitude.rawValue)|\(horizon.rawValue)|\(reasonSummary)|\(lines)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Proof Summary Model

/// View model for the /proof screen.
struct ProofSummaryModel: Hashable {
    let statement: String
    let categories: [Category]
    let magnitude: MagnitudeBucket
    let whyLine: String

    func canonicalString() -> String {
        let cats = categories.map(\.rawValue).sorted().joined(separator: ",")
        return "PROOF|v1|\(magnitude.rawValue)|\(cats)|\(statement)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Connection State Model

/// View model for connection state.
struct ConnectionStateModel: Hashable {
    let kind: ConnectionKind
    let status: ConnectionStatus
}

// MARK: - Mirror Source Summary

/// View model for a single source in /mirror.
struct MirrorSourceSummary: Hashable {
    let kind: ConnectionKind
    let readSuccessfully: Bool
    let notStored: [String]
    let observed: [ObservedItem]
}

/// Single observed item.
struct ObservedItem: Hashable {
    let category: ObservedCategory
    let magnitude: MagnitudeBucket
    let horizon: HorizonBucket
}

// MARK: - Mirror Outcome Model

/// What changed as a result.
struct MirrorOutcomeModel: Hashable {
    let heldQuietly: Bool
    let heldMagnitude: MagnitudeBucket
    let nothingRequiresAttention: Bool
}

// MARK: - Mirror Page Model

/// View model for the /mirror screen.
struct MirrorPageModel: Hashable {
    let title: String
    let subtitle: String
    let sources: [MirrorSourceSummary]
    let outcome: MirrorOutcomeModel
    let restraintStatement: String
    let restraintWhy: String

    func canonicalString() -> String {
        let srcStrings = sources.map { src in
            let obs = src.observed.map { "\($0.category.rawValue):\($0.magnitude.rawValue)" }.joined(separator: ";")
            return "\(src.kind.rawValue):\(src.readSuccessfully):\(obs)"
        }.joined(separator: "|")
        return "MIRROR|v1|\(title)|\(srcStrings)|\(outcome.heldMagnitude.rawValue)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Landing Moment

/// A single moment on the landing page.
struct LandingMoment: Hashable, Identifiable {
    let id: String
    let headline: String
    let body: String
}

// MARK: - Landing Page Model

/// View model for the landing page.
struct LandingPageModel: Hashable {
    let moments: [LandingMoment]
    let interestPlaceholder: String

    func canonicalString() -> String {
        let momentStrings = moments.map { "\($0.id):\($0.headline)" }.joined(separator: "|")
        return "LANDING|v1|\(momentStrings)"
    }

    func hash256() -> String {
        let data = Data(canonicalString().utf8)
        let digest = SHA256.hash(data: data)
        return digest.map { String(format: "%02x", $0) }.joined()
    }
}
