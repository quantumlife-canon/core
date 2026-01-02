// SeededData.swift
// Deterministic mock data generator for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Same seed MUST produce identical output.
// CRITICAL: No random calls - use seed-based selection.
// CRITICAL: No identifiers (names, emails, dates, amounts).

import Foundation
import CryptoKit

// MARK: - Seeded Data Generator

/// Generates deterministic mock data from a seed string.
/// Same seed = same output, always.
final class SeededDataGenerator {

    /// The seed string used for generation.
    let seed: String

    /// Default seed for demo mode.
    static let defaultSeed = "demo-seed-v1-2025"

    /// The seed hash used for deterministic selection.
    private let seedHash: [UInt8]

    init(seed: String = SeededDataGenerator.defaultSeed) {
        self.seed = seed
        let data = Data(seed.utf8)
        let digest = SHA256.hash(data: data)
        self.seedHash = Array(digest)
    }

    // MARK: - Deterministic Selection

    /// Selects an index deterministically based on seed position.
    private func selectIndex(position: Int, count: Int) -> Int {
        guard count > 0 else { return 0 }
        let byte = seedHash[position % seedHash.count]
        return Int(byte) % count
    }

    /// Selects a boolean deterministically based on seed position.
    private func selectBool(position: Int) -> Bool {
        let byte = seedHash[position % seedHash.count]
        return byte % 2 == 0
    }

    /// Selects a count (0-5) deterministically based on seed position.
    private func selectCount(position: Int) -> Int {
        let byte = seedHash[position % seedHash.count]
        return Int(byte) % 6
    }

    // MARK: - Landing Page

    func generateLandingPage() -> LandingPageModel {
        let moments = [
            LandingMoment(
                id: "arrival",
                headline: "Nothing needs you.",
                body: "Right now, everything that could need you has been considered. Nothing does."
            ),
            LandingMoment(
                id: "recognition",
                headline: "If something did,",
                body: "You'd know. Not through noise. Through quiet certainty."
            ),
            LandingMoment(
                id: "promise",
                headline: "QuantumLife watches.",
                body: "Email, calendar, the things that pile up. We hold them until they become obligations — or disappear."
            ),
            LandingMoment(
                id: "permission",
                headline: "You can ignore this.",
                body: "Most people do. That's the point."
            )
        ]

        return LandingPageModel(
            moments: moments,
            interestPlaceholder: "Enter your email"
        )
    }

    // MARK: - Today Page

    func generateTodayPage(preference: PreferenceMode) -> TodayPageModel {
        let observations = [
            "Everything that could need you has been considered.",
            "There's nothing you need to do right now.",
            "We're watching, so you don't have to."
        ]

        // Determine whisper cue based on preference and seed
        var whisperCue: WhisperCue? = nil

        if preference == .quiet {
            // Single whisper rule: surface > proof
            let hasSurface = selectBool(position: 0)
            let hasProof = selectBool(position: 1)

            if hasSurface {
                whisperCue = WhisperCue(
                    type: .surface,
                    text: "If you wanted to, there's one thing you could look at.",
                    linkText: "View, if you like"
                )
            } else if hasProof {
                whisperCue = WhisperCue(
                    type: .proof,
                    text: "If you ever wondered—quiet is being kept.",
                    linkText: "Proof, if you want it."
                )
            }
        }

        return TodayPageModel(
            greeting: "Today, quietly.",
            observations: observations,
            whisperCue: whisperCue,
            preference: preference
        )
    }

    // MARK: - Held Summary

    func generateHeldSummary() -> HeldSummaryModel {
        // Select categories based on seed
        var categories: [Category] = []
        if selectBool(position: 2) { categories.append(.time) }
        if selectBool(position: 3) { categories.append(.money) }
        if selectBool(position: 4) { categories.append(.work) }

        // Determine magnitude
        let count = categories.count
        let magnitude = MagnitudeBucket.from(count: count + selectCount(position: 5))

        // Select statement based on magnitude
        let statement: String
        switch magnitude {
        case .none:
            statement = "Everything that could need you has been considered. Nothing does."
        case .aFew:
            statement = "There are a few things we're holding quietly for you. None of them need you today."
        case .several:
            statement = "We're holding several things quietly. None of them are urgent."
        }

        return HeldSummaryModel(
            statement: statement,
            categories: categories.isEmpty ? [.time] : categories,
            magnitude: magnitude == .none ? .aFew : magnitude
        )
    }

    // MARK: - Surface Item

    func generateSurfaceItem() -> SurfaceItemModel? {
        // Only show surface item sometimes
        if !selectBool(position: 0) {
            return nil
        }

        // Select category deterministically
        let categoryIndex = selectIndex(position: 6, count: Category.allCases.count)
        let category = Category.allCases[categoryIndex]

        // Reason summaries by category
        let reasonSummaries: [Category: String] = [
            .money: "We noticed a pattern that tends to become urgent if ignored.",
            .time: "Something time-related is being held that you might want to know about.",
            .work: "There's a work-related item we're watching quietly.",
            .people: "We're holding something related to people in your life.",
            .home: "A household matter is being held for you."
        ]

        // Explain lines by category
        let explainLines: [Category: [String]] = [
            .money: [
                "This category has shown patterns before.",
                "We're watching it so you don't have to.",
                "No action is required from you."
            ],
            .time: [
                "Time-sensitive items are being monitored.",
                "Nothing needs your attention right now.",
                "We'll surface it if it becomes urgent."
            ],
            .work: [
                "Work items are being held quietly.",
                "We noticed activity that may be relevant later.",
                "You can ignore this completely if you prefer."
            ],
            .people: [
                "People-related items are being watched.",
                "Nothing requires your response.",
                "We're here if you want to look."
            ],
            .home: [
                "Household matters are being tracked.",
                "Everything is under control.",
                "View only if you're curious."
            ]
        ]

        // Determine horizon based on category
        let horizon: HorizonBucket
        switch category {
        case .money: horizon = .recent
        case .time: horizon = .ongoing
        case .work: horizon = .ongoing
        case .people: horizon = .earlier
        case .home: horizon = .earlier
        }

        // Generate item key hash
        let itemKeyData = Data("item|\(category.rawValue)|\(seed)".utf8)
        let itemKeyHash = SHA256.hash(data: itemKeyData).map { String(format: "%02x", $0) }.joined()

        return SurfaceItemModel(
            category: category,
            magnitude: .aFew,
            horizon: horizon,
            reasonSummary: reasonSummaries[category] ?? "",
            explainLines: explainLines[category] ?? [],
            itemKeyHash: String(itemKeyHash.prefix(16))
        )
    }

    // MARK: - Proof Summary

    func generateProofSummary() -> ProofSummaryModel {
        // Select categories based on seed
        var categories: [Category] = []
        if selectBool(position: 7) { categories.append(.money) }
        if selectBool(position: 8) { categories.append(.time) }
        if selectBool(position: 9) { categories.append(.work) }

        // Ensure at least one category
        if categories.isEmpty {
            categories = [.time]
        }

        // Determine magnitude
        let magnitude = MagnitudeBucket.from(count: categories.count + 1)

        // Select statement based on magnitude
        let statement: String
        switch magnitude {
        case .none:
            statement = "Nothing needed holding."
        case .aFew:
            statement = "We chose not to interrupt you a few times."
        case .several:
            statement = "We chose not to interrupt you often."
        }

        return ProofSummaryModel(
            statement: statement,
            categories: categories,
            magnitude: magnitude,
            whyLine: "Quiet is a feature. Not a gap."
        )
    }

    // MARK: - Connection States

    func generateConnectionStates() -> [ConnectionStateModel] {
        return ConnectionKind.allCases.map { kind in
            // Determine status based on seed and kind
            let position = 10 + ConnectionKind.allCases.firstIndex(of: kind)!
            let isConnected = selectBool(position: position)

            return ConnectionStateModel(
                kind: kind,
                status: isConnected ? .connectedMock : .notConnected
            )
        }
    }

    // MARK: - Mirror Page

    func generateMirrorPage(connections: [ConnectionStateModel]) -> MirrorPageModel {
        let connectedSources = connections.filter { $0.status.isConnected }

        // Not stored statements by kind
        let notStoredByKind: [ConnectionKind: [String]] = [
            .email: ["messages", "senders", "subjects"],
            .calendar: ["event details", "attendees", "locations"],
            .finance: ["account numbers", "specific amounts", "vendor details"]
        ]

        // Generate source summaries
        let sources: [MirrorSourceSummary] = connectedSources.map { conn in
            var observed: [ObservedItem] = []

            switch conn.kind {
            case .email:
                observed = [
                    ObservedItem(category: .timeCommitments, magnitude: .aFew, horizon: .ongoing),
                    ObservedItem(category: .receipts, magnitude: .aFew, horizon: .recent)
                ]
            case .calendar:
                observed = [
                    ObservedItem(category: .timeCommitments, magnitude: .several, horizon: .ongoing)
                ]
            case .finance:
                observed = [
                    ObservedItem(category: .receipts, magnitude: .aFew, horizon: .recent),
                    ObservedItem(category: .patterns, magnitude: .aFew, horizon: .ongoing)
                ]
            }

            return MirrorSourceSummary(
                kind: conn.kind,
                readSuccessfully: true,
                notStored: notStoredByKind[conn.kind] ?? [],
                observed: observed
            )
        }

        // Determine outcome
        let hasHeld = selectBool(position: 15)
        let outcome = MirrorOutcomeModel(
            heldQuietly: hasHeld,
            heldMagnitude: hasHeld ? .aFew : .none,
            nothingRequiresAttention: true
        )

        return MirrorPageModel(
            title: "Seen, quietly.",
            subtitle: "A record of what we noticed — and what we didn't keep.",
            sources: sources,
            outcome: outcome,
            restraintStatement: "We chose not to interrupt you.",
            restraintWhy: "Quiet is a feature, not a gap."
        )
    }
}

// MARK: - Shared Instance

extension SeededDataGenerator {
    /// Shared generator with default seed for app-wide use.
    static let shared = SeededDataGenerator()
}
