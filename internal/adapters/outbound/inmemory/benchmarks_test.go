//go:build dev

package inmemory_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// BenchmarkIssueIdentity benchmarks the performance invariant:
// Identity document issuance should complete in <10ms for reasonable load
func BenchmarkIssueIdentity(b *testing.B) {
	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	credential := domain.NewIdentityCredentialFromComponents(
		domain.NewTrustDomainFromName("example.org"), "/workload")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.IssueIdentity(ctx, credential)
		if err != nil {
			b.Fatalf("IssueIdentity failed: %v", err)
		}
	}
}

// BenchmarkMatchesSelectors benchmarks selector matching performance:
// Matching should be O(n*m) where n=mapper selectors, m=workload selectors
func BenchmarkMatchesSelectors(b *testing.B) {
	// Setup mapper with varying selector counts
	benchmarks := []struct {
		name             string
		mapperSelCount   int
		workloadSelCount int
	}{
		{"1mapper_1workload", 1, 1},
		{"3mapper_5workload", 3, 5},
		{"5mapper_10workload", 5, 10},
		{"10mapper_20workload", 10, 20},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Create mapper with N selectors
			td := domain.NewTrustDomainFromName("example.org")
			credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

			mapperSelectors := domain.NewSelectorSet()
			for i := 0; i < bm.mapperSelCount; i++ {
				sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+i)))
				mapperSelectors.Add(sel)
			}

			mapper, err := domain.NewIdentityMapper(credential, mapperSelectors)
			if err != nil {
				b.Fatalf("Failed to create mapper: %v", err)
			}

			// Create workload with M selectors (includes all mapper selectors)
			workloadSelectors := domain.NewSelectorSet()
			for i := 0; i < bm.workloadSelCount; i++ {
				sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+i)))
				workloadSelectors.Add(sel)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = mapper.MatchesSelectors(workloadSelectors)
			}
		})
	}
}

// BenchmarkRegistryFindBySelectors benchmarks registry lookup performance:
// Should remain efficient with 100s of mappers
func BenchmarkRegistryFindBySelectors(b *testing.B) {
	ctx := context.Background()

	benchmarks := []struct {
		name          string
		mapperCount   int
		selectorCount int
	}{
		{"10mappers_1sel", 10, 1},
		{"100mappers_1sel", 100, 1},
		{"100mappers_3sels", 100, 3},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			registry := inmemory.NewInMemoryRegistry()
			td := domain.NewTrustDomainFromName("example.org")

			// Seed registry with N mappers
			for i := 0; i < bm.mapperCount; i++ {
				credential := domain.NewIdentityCredentialFromComponents(td, "/workload"+string(rune('0'+i)))
				selectors := domain.NewSelectorSet()
				for j := 0; j < bm.selectorCount; j++ {
					sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+i)) + string(rune('0'+j)))
					selectors.Add(sel)
				}
				mapper, _ := domain.NewIdentityMapper(credential, selectors)
				_ = registry.Seed(ctx, mapper)
			}
			registry.Seal()

			// Search for last mapper's selectors (worst case)
			searchSelectors := domain.NewSelectorSet()
			for j := 0; j < bm.selectorCount; j++ {
				sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+bm.mapperCount-1)) + string(rune('0'+j)))
				searchSelectors.Add(sel)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = registry.FindBySelectors(ctx, searchSelectors)
			}
		})
	}
}

// BenchmarkSelectorSetAdd benchmarks set insertion performance:
// Should remain O(n) for uniqueness check
func BenchmarkSelectorSetAdd(b *testing.B) {
	benchmarks := []struct {
		name    string
		setSize int
	}{
		{"empty_set", 0},
		{"10_items", 10},
		{"100_items", 100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Pre-populate set
			set := domain.NewSelectorSet()
			for i := 0; i < bm.setSize; i++ {
				sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+i)))
				set.Add(sel)
			}

			// Benchmark adding new (unique) selector
			newSel, _ := domain.ParseSelectorFromString("unix:gid:9999")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Reset between iterations (since Add is idempotent)
				if i > 0 {
					set = domain.NewSelectorSet()
					for j := 0; j < bm.setSize; j++ {
						sel, _ := domain.ParseSelectorFromString("unix:uid:" + string(rune('0'+j)))
						set.Add(sel)
					}
				}
				set.Add(newSel)
			}
		})
	}
}

// BenchmarkSelectorParse benchmarks selector parsing performance:
// Should be fast for common formats
func BenchmarkSelectorParse(b *testing.B) {
	benchmarks := []struct {
		name     string
		selector string
	}{
		{"simple", "unix:uid:1000"},
		{"k8s_namespace", "k8s:namespace:production"},
		{"multi_colon", "custom:key:value:with:colons:here"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := domain.ParseSelectorFromString(bm.selector)
				if err != nil {
					b.Fatalf("Parse failed: %v", err)
				}
			}
		})
	}
}
