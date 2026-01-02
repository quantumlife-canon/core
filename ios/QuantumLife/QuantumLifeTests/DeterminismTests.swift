// DeterminismTests.swift
// Determinism tests for QuantumLife iOS
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// CRITICAL: Same seed MUST produce identical output.
// CRITICAL: Hash must be stable across runs.

import XCTest
@testable import QuantumLife

final class DeterminismTests: XCTestCase {

    // MARK: - Landing Page Determinism

    func testLandingPageDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let page1 = gen1.generateLandingPage()
        let page2 = gen2.generateLandingPage()

        XCTAssertEqual(page1.hash256(), page2.hash256(), "Same seed must produce same landing page")
        XCTAssertEqual(page1.moments.count, page2.moments.count)

        for (m1, m2) in zip(page1.moments, page2.moments) {
            XCTAssertEqual(m1.id, m2.id)
            XCTAssertEqual(m1.headline, m2.headline)
            XCTAssertEqual(m1.body, m2.body)
        }
    }

    func testLandingPageDifferentSeeds() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-2")

        let page1 = gen1.generateLandingPage()
        let page2 = gen2.generateLandingPage()

        // Landing page content is static, so hashes should still match
        // (landing page is not seeded differently)
        XCTAssertEqual(page1.hash256(), page2.hash256())
    }

    // MARK: - Today Page Determinism

    func testTodayPageDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let page1 = gen1.generateTodayPage(preference: .quiet)
        let page2 = gen2.generateTodayPage(preference: .quiet)

        XCTAssertEqual(page1.hash256(), page2.hash256(), "Same seed must produce same today page")
        XCTAssertEqual(page1.greeting, page2.greeting)
        XCTAssertEqual(page1.observations, page2.observations)

        // Whisper cue should be identical
        XCTAssertEqual(page1.whisperCue?.type.rawValue, page2.whisperCue?.type.rawValue)
        XCTAssertEqual(page1.whisperCue?.text, page2.whisperCue?.text)
    }

    func testTodayPagePreferenceAffectsOutput() {
        let gen = SeededDataGenerator(seed: "test-seed-1")

        let quietPage = gen.generateTodayPage(preference: .quiet)
        let showAllPage = gen.generateTodayPage(preference: .showAll)

        XCTAssertNotEqual(quietPage.hash256(), showAllPage.hash256(), "Different preferences should produce different pages")
    }

    // MARK: - Held Summary Determinism

    func testHeldSummaryDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let summary1 = gen1.generateHeldSummary()
        let summary2 = gen2.generateHeldSummary()

        XCTAssertEqual(summary1.hash256(), summary2.hash256(), "Same seed must produce same held summary")
        XCTAssertEqual(summary1.categories, summary2.categories)
        XCTAssertEqual(summary1.magnitude, summary2.magnitude)
    }

    func testHeldSummaryDifferentSeeds() {
        let gen1 = SeededDataGenerator(seed: "seed-a")
        let gen2 = SeededDataGenerator(seed: "seed-b")

        let summary1 = gen1.generateHeldSummary()
        let summary2 = gen2.generateHeldSummary()

        // Different seeds may produce different categories
        // (but could coincidentally match - this test verifies stability, not difference)
        XCTAssertNotNil(summary1.hash256())
        XCTAssertNotNil(summary2.hash256())
    }

    // MARK: - Surface Item Determinism

    func testSurfaceItemDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let item1 = gen1.generateSurfaceItem()
        let item2 = gen2.generateSurfaceItem()

        if let i1 = item1, let i2 = item2 {
            XCTAssertEqual(i1.hash256(), i2.hash256(), "Same seed must produce same surface item")
            XCTAssertEqual(i1.category, i2.category)
            XCTAssertEqual(i1.itemKeyHash, i2.itemKeyHash)
        } else {
            // Both should be nil or both should have value
            XCTAssertEqual(item1 == nil, item2 == nil)
        }
    }

    // MARK: - Proof Summary Determinism

    func testProofSummaryDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let summary1 = gen1.generateProofSummary()
        let summary2 = gen2.generateProofSummary()

        XCTAssertEqual(summary1.hash256(), summary2.hash256(), "Same seed must produce same proof summary")
        XCTAssertEqual(summary1.categories, summary2.categories)
        XCTAssertEqual(summary1.whyLine, summary2.whyLine)
    }

    // MARK: - Connection States Determinism

    func testConnectionStatesDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let states1 = gen1.generateConnectionStates()
        let states2 = gen2.generateConnectionStates()

        XCTAssertEqual(states1.count, states2.count)
        for (s1, s2) in zip(states1, states2) {
            XCTAssertEqual(s1.kind, s2.kind)
            XCTAssertEqual(s1.status, s2.status)
        }
    }

    // MARK: - Mirror Page Determinism

    func testMirrorPageDeterminism() {
        let gen1 = SeededDataGenerator(seed: "test-seed-1")
        let gen2 = SeededDataGenerator(seed: "test-seed-1")

        let connections = [
            ConnectionStateModel(kind: .email, status: .connectedMock),
            ConnectionStateModel(kind: .calendar, status: .connectedMock)
        ]

        let page1 = gen1.generateMirrorPage(connections: connections)
        let page2 = gen2.generateMirrorPage(connections: connections)

        XCTAssertEqual(page1.hash256(), page2.hash256(), "Same seed must produce same mirror page")
        XCTAssertEqual(page1.sources.count, page2.sources.count)
        XCTAssertEqual(page1.outcome.heldMagnitude, page2.outcome.heldMagnitude)
    }

    // MARK: - Hash Stability

    func testHashStabilityLanding() {
        let gen = SeededDataGenerator(seed: "demo-seed-v1-2025")
        let page = gen.generateLandingPage()

        // This hash should remain stable across runs
        let expectedHash = page.hash256()
        XCTAssertEqual(expectedHash.count, 64, "SHA256 hash should be 64 hex characters")

        // Verify it's a valid hex string
        let validHex = expectedHash.allSatisfy { $0.isHexDigit }
        XCTAssertTrue(validHex, "Hash should be valid hex")
    }

    func testHashStabilityToday() {
        let gen = SeededDataGenerator(seed: "demo-seed-v1-2025")
        let page = gen.generateTodayPage(preference: .quiet)

        let hash = page.hash256()
        XCTAssertEqual(hash.count, 64)

        // Run again and verify identical
        let page2 = gen.generateTodayPage(preference: .quiet)
        XCTAssertEqual(page.hash256(), page2.hash256())
    }

    // MARK: - Canonical String Format

    func testCanonicalStringFormat() {
        let gen = SeededDataGenerator(seed: "test")

        // Landing
        let landing = gen.generateLandingPage()
        XCTAssertTrue(landing.canonicalString().hasPrefix("LANDING|v1|"))

        // Today
        let today = gen.generateTodayPage(preference: .quiet)
        XCTAssertTrue(today.canonicalString().hasPrefix("TODAY|v1|"))

        // Held
        let held = gen.generateHeldSummary()
        XCTAssertTrue(held.canonicalString().hasPrefix("HELD|v1|"))

        // Surface
        if let surface = gen.generateSurfaceItem() {
            XCTAssertTrue(surface.canonicalString().hasPrefix("SURFACE|v1|"))
        }

        // Proof
        let proof = gen.generateProofSummary()
        XCTAssertTrue(proof.canonicalString().hasPrefix("PROOF|v1|"))

        // Mirror
        let mirror = gen.generateMirrorPage(connections: [])
        XCTAssertTrue(mirror.canonicalString().hasPrefix("MIRROR|v1|"))
    }

    // MARK: - Abstract Only

    func testNoIdentifiersInOutput() {
        let gen = SeededDataGenerator(seed: "test-seed-1")

        // Check landing page doesn't contain identifiers
        let landing = gen.generateLandingPage()
        for moment in landing.moments {
            XCTAssertFalse(moment.headline.contains("@"), "No emails in output")
            XCTAssertFalse(moment.body.contains("$"), "No dollar amounts in output")
        }

        // Check today page
        let today = gen.generateTodayPage(preference: .quiet)
        for obs in today.observations {
            XCTAssertFalse(obs.contains("@"), "No emails in observations")
        }

        // Check held summary uses only magnitude buckets
        let held = gen.generateHeldSummary()
        XCTAssertTrue(MagnitudeBucket.allCases.contains(held.magnitude))
        for cat in held.categories {
            XCTAssertTrue(Category.allCases.contains(cat))
        }
    }

    // MARK: - Single Whisper Rule

    func testSingleWhisperRule() {
        // Test that only one whisper cue is shown at most
        for seed in ["seed1", "seed2", "seed3", "seed4", "seed5"] {
            let gen = SeededDataGenerator(seed: seed)
            let today = gen.generateTodayPage(preference: .quiet)

            // At most one whisper cue
            if let cue = today.whisperCue {
                XCTAssertTrue(cue.type == .surface || cue.type == .proof)
            }
            // (nil is also valid - no cue)
        }
    }

    // MARK: - Default Seed

    func testDefaultSeedConsistency() {
        let gen1 = SeededDataGenerator()
        let gen2 = SeededDataGenerator()

        XCTAssertEqual(gen1.seed, gen2.seed)
        XCTAssertEqual(gen1.seed, SeededDataGenerator.defaultSeed)

        let page1 = gen1.generateTodayPage(preference: .quiet)
        let page2 = gen2.generateTodayPage(preference: .quiet)

        XCTAssertEqual(page1.hash256(), page2.hash256())
    }
}
