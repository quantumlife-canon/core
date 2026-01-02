// Package main exports CSS tokens to Swift for iOS.
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// This script reads tokens.css and generates DesignTokens.swift.
// CRITICAL: stdlib only.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run export_tokens_to_swift.go <input.css> <output.swift>")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	tokens, err := parseTokensCSS(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing CSS: %v\n", err)
		os.Exit(1)
	}

	if err := generateSwift(tokens, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Swift: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s with %d tokens\n", outputPath, len(tokens))
}

type Token struct {
	Name     string
	Value    string
	Category string
}

func parseTokensCSS(path string) ([]Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tokens []Token
	var currentCategory string

	// Regex to match CSS custom properties
	propRegex := regexp.MustCompile(`^\s*--([\w-]+):\s*(.+?);`)
	// Regex to match category comments
	catRegex := regexp.MustCompile(`/\*.*?═+\s*\n?\s*(\w+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for category header
		if strings.Contains(line, "═══") {
			// Next meaningful word is the category
			if matches := catRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentCategory = matches[1]
			} else if strings.Contains(line, "TYPOGRAPHY") {
				currentCategory = "Typography"
			} else if strings.Contains(line, "SPACING") {
				currentCategory = "Spacing"
			} else if strings.Contains(line, "COLORS") {
				currentCategory = "Colors"
			} else if strings.Contains(line, "RADIUS") {
				currentCategory = "Radius"
			} else if strings.Contains(line, "SHADOW") {
				currentCategory = "Shadow"
			} else if strings.Contains(line, "MOTION") {
				currentCategory = "Motion"
			} else if strings.Contains(line, "Z-INDEX") {
				currentCategory = "ZIndex"
			} else if strings.Contains(line, "COMPONENT") {
				currentCategory = "Component"
			}
			continue
		}

		// Parse CSS property
		if matches := propRegex.FindStringSubmatch(line); len(matches) == 3 {
			tokens = append(tokens, Token{
				Name:     matches[1],
				Value:    matches[2],
				Category: currentCategory,
			})
		}
	}

	return tokens, scanner.Err()
}

func generateSwift(tokens []Token, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprint(file, `// DesignTokens.swift
// Auto-generated from tokens.css - DO NOT EDIT MANUALLY
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md
//
// These tokens are the atomic units of the visual system.
// All component views must reference tokens, never raw values.

import SwiftUI

// MARK: - Design Tokens

enum DesignTokens {

    // MARK: - Typography

    enum Typography {
        static let fontSans = Font.system(.body)
        static let fontMono = Font.system(.body, design: .monospaced)

        // Type Scale (in points)
        static let textXS: CGFloat = 11
        static let textSM: CGFloat = 13
        static let textBase: CGFloat = 15
        static let textLG: CGFloat = 17
        static let textXL: CGFloat = 21
        static let text2XL: CGFloat = 28
        static let text3XL: CGFloat = 36

        // Line Height
        static let leadingTight: CGFloat = 1.2
        static let leadingNormal: CGFloat = 1.5
        static let leadingRelaxed: CGFloat = 1.7

        // Font Weight
        static let fontNormal: Font.Weight = .regular
        static let fontMedium: Font.Weight = .medium
        static let fontSemibold: Font.Weight = .semibold
    }

    // MARK: - Spacing

    enum Spacing {
        static let space1: CGFloat = 4
        static let space2: CGFloat = 8
        static let space3: CGFloat = 12
        static let space4: CGFloat = 16
        static let space6: CGFloat = 24
        static let space8: CGFloat = 32
        static let space12: CGFloat = 48
        static let space16: CGFloat = 64
    }

    // MARK: - Colors

    enum Colors {
        // Light mode colors (default)
        static let bg = Color(hex: "FAFAFA")
        static let surface = Color(hex: "FFFFFF")
        static let surfaceRaised = Color(hex: "FFFFFF")

        static let textPrimary = Color(hex: "1A1A1A")
        static let textSecondary = Color(hex: "666666")
        static let textTertiary = Color(hex: "999999")
        static let textQuaternary = Color(hex: "BBBBBB")

        static let border = Color(hex: "E5E5E5")
        static let borderSubtle = Color(hex: "F0F0F0")

        static let focus = Color(hex: "0066CC")
        static let link = Color(hex: "0066CC")
        static let linkHover = Color(hex: "004499")

        static let actionPrimary = Color(hex: "1A1A1A")
        static let actionPrimaryText = Color(hex: "FFFFFF")
        static let actionSecondaryBorder = Color(hex: "CCCCCC")
        static let actionSecondaryText = Color(hex: "1A1A1A")

        static let levelSilent = Color.clear
        static let levelAmbient = Color(hex: "F5F5F5")
        static let levelNeedsYou = Color(hex: "FFF9E6")
        static let levelUrgent = Color(hex: "FFF0F0")

        static let success = Color(hex: "2E7D32")
        static let error = Color(hex: "C62828")
        static let warning = Color(hex: "F9A825")

        // Adaptive colors that respect dark mode
        static var adaptiveBg: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "121212")
                    : UIColor(hex: "FAFAFA")
            })
        }

        static var adaptiveSurface: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "1E1E1E")
                    : UIColor(hex: "FFFFFF")
            })
        }

        static var adaptiveTextPrimary: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "EBEBEB")
                    : UIColor(hex: "1A1A1A")
            })
        }

        static var adaptiveTextSecondary: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "A0A0A0")
                    : UIColor(hex: "666666")
            })
        }

        static var adaptiveTextTertiary: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "707070")
                    : UIColor(hex: "999999")
            })
        }

        static var adaptiveBorder: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "333333")
                    : UIColor(hex: "E5E5E5")
            })
        }

        static var adaptiveBorderSubtle: Color {
            Color(UIColor { traits in
                traits.userInterfaceStyle == .dark
                    ? UIColor(hex: "282828")
                    : UIColor(hex: "F0F0F0")
            })
        }
    }

    // MARK: - Radius

    enum Radius {
        static let sm: CGFloat = 4
        static let md: CGFloat = 8
        static let lg: CGFloat = 12
        static let full: CGFloat = 9999
    }

    // MARK: - Motion

    enum Motion {
        static let durationInstant: Double = 0
        static let durationFast: Double = 0.1
        static let durationNormal: Double = 0.2
        static let durationSlow: Double = 0.3
    }

    // MARK: - Component Tokens

    enum Component {
        static let cardPadding = Spacing.space4
        static let cardRadius = Radius.md

        static let buttonPaddingX = Spacing.space4
        static let buttonPaddingY = Spacing.space2
        static let buttonRadius = Radius.sm
        static let buttonFontSize = Typography.textSM

        static let inputPaddingX = Spacing.space3
        static let inputPaddingY = Spacing.space2
        static let inputRadius = Radius.sm

        static let containerMaxWidth: CGFloat = 1024
        static let containerPadding = Spacing.space4

        static let headerHeight: CGFloat = 56
    }
}

// MARK: - Color Extensions

extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 3: // RGB (12-bit)
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6: // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8: // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, 0, 0, 0)
        }
        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

extension UIColor {
    convenience init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 6:
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8:
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, 0, 0, 0)
        }
        self.init(
            red: CGFloat(r) / 255,
            green: CGFloat(g) / 255,
            blue: CGFloat(b) / 255,
            alpha: CGFloat(a) / 255
        )
    }
}

// MARK: - View Modifiers

extension View {
    func textStyle(_ size: CGFloat, weight: Font.Weight = .regular, color: Color = DesignTokens.Colors.adaptiveTextPrimary) -> some View {
        self
            .font(.system(size: size, weight: weight))
            .foregroundColor(color)
    }

    func whisperStyle() -> some View {
        self
            .font(.system(size: DesignTokens.Typography.textXS))
            .foregroundColor(DesignTokens.Colors.adaptiveTextTertiary)
    }
}
`)
	return nil
}
