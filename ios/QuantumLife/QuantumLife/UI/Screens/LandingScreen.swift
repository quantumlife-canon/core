// LandingScreen.swift
// Landing page with Moments
//
// Phase 19.0: iOS Shell
// Reference: docs/ADR/ADR-0040-phase19-ios-shell.md

import SwiftUI

struct LandingScreen: View {
    @State private var email = ""
    @State private var interestSubmitted = false

    private let model = SeededDataGenerator.shared.generateLandingPage()

    var body: some View {
        ScrollView {
            VStack(spacing: DesignTokens.Spacing.space16) {
                // Moments
                ForEach(model.moments) { moment in
                    MomentView(moment: moment)
                }

                // Interest capture (mocked)
                if !interestSubmitted {
                    VStack(spacing: DesignTokens.Spacing.space4) {
                        TextField(model.interestPlaceholder, text: $email)
                            .textFieldStyle(QLTextFieldStyle())
                            .autocapitalization(.none)
                            .keyboardType(.emailAddress)

                        QLButton("Get early access", style: .primary) {
                            interestSubmitted = true
                        }
                    }
                    .padding(.horizontal, DesignTokens.Spacing.space8)
                } else {
                    Text("Thank you. We'll be in touch.")
                        .font(.system(size: DesignTokens.Typography.textSM))
                        .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                }

                // Subtle link to Start
                WhisperLink("Start") {
                    // Navigate to Start
                }
                .padding(.top, DesignTokens.Spacing.space8)
            }
            .padding(DesignTokens.Spacing.space6)
        }
        .background(DesignTokens.Colors.adaptiveBg)
    }
}

// MARK: - Moment View

struct MomentView: View {
    let moment: LandingMoment

    var body: some View {
        VStack(alignment: .center, spacing: DesignTokens.Spacing.space3) {
            Text(moment.headline)
                .font(.system(size: DesignTokens.Typography.text2XL, weight: DesignTokens.Typography.fontNormal))
                .foregroundColor(DesignTokens.Colors.adaptiveTextPrimary)
                .multilineTextAlignment(.center)

            Text(moment.body)
                .font(.system(size: DesignTokens.Typography.textBase))
                .foregroundColor(DesignTokens.Colors.adaptiveTextSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, DesignTokens.Spacing.space4)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, DesignTokens.Spacing.space12)
    }
}

// MARK: - Text Field Style

struct QLTextFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<Self._Label>) -> some View {
        configuration
            .font(.system(size: DesignTokens.Typography.textSM))
            .padding(.horizontal, DesignTokens.Component.inputPaddingX)
            .padding(.vertical, DesignTokens.Component.inputPaddingY)
            .background(DesignTokens.Colors.adaptiveSurface)
            .cornerRadius(DesignTokens.Component.inputRadius)
            .overlay(
                RoundedRectangle(cornerRadius: DesignTokens.Component.inputRadius)
                    .stroke(DesignTokens.Colors.adaptiveBorder, lineWidth: 1)
            )
    }
}
