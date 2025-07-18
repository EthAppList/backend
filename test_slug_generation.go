package main

import (
	"fmt"
	"regexp"
	"strings"
)

// generateSlugLocal generates a URL-friendly slug from a given text (local implementation for testing)
func generateSlugLocal(inputText string) string {
	// Remove special characters except alphanumeric, spaces, and hyphens
	re1 := regexp.MustCompile(`[^a-zA-Z0-9\s\-]`)
	cleaned := re1.ReplaceAllString(strings.TrimSpace(inputText), "")

	// Replace multiple spaces with single hyphens
	re2 := regexp.MustCompile(`\s+`)
	slug := re2.ReplaceAllString(cleaned, "-")

	return strings.ToLower(slug)
}

func main() {
	fmt.Println("Testing slug generation for categories and chains...")

	// Test categories
	categories := []string{
		"DeFi",
		"NFT",
		"GameFi",
		"Infrastructure",
		"Social",
		"DAO",
		"Privacy",
		"Decentralized Exchange",
		"Yield Farming",
		"Cross-Chain Bridges",
	}

	fmt.Println("\nCategory Slugs:")
	for _, category := range categories {
		slug := generateSlugLocal(category)
		fmt.Printf("%-25s -> %s\n", category, slug)
	}

	// Test chains
	chains := []string{
		"Ethereum",
		"Polygon",
		"Solana",
		"Binance Smart Chain",
		"Arbitrum",
		"Optimism",
		"Avalanche",
		"Base",
		"Celo",
		"Fantom",
		"Gnosis",
		"Moonbeam",
		"Cronos",
		"zkSync",
		"Linea",
		"Metis",
		"Polygon zkEVM",
		"Arbitrum Nova",
	}

	fmt.Println("\nChain Slugs:")
	for _, chain := range chains {
		slug := generateSlugLocal(chain)
		fmt.Printf("%-25s -> %s\n", chain, slug)
	}

	// Test edge cases
	fmt.Println("\nEdge Cases:")
	edgeCases := []string{
		"DeFi 2.0",
		"Layer-2 Solutions",
		"Web3 & Crypto",
		"NFT Marketplace",
		"Cross Chain",
		"Multi-Chain",
		"   Spaced Out   ",
		"Special!@#$%Characters",
	}

	for _, edge := range edgeCases {
		slug := generateSlugLocal(edge)
		fmt.Printf("%-25s -> %s\n", edge, slug)
	}
}
