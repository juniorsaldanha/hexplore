package enrich

import "github.com/juniorsaldanha/hexplore/internal/domain"

const blockVSize = 1_000_000 // one block, in vbytes

// ProjectNextBlocks derives up to n not-yet-mined blocks from a mempool fee
// histogram (docs/PLAN.md §5): [feerate, vsize] pairs descending by feerate.
// Coarse by construction — only ~7-10 histogram buckets exist — every result
// is marked Approx. Providers with a native projection (Caps().NextBlockExact)
// should use that instead of calling this.
func ProjectNextBlocks(m domain.MempoolState, n int) []domain.ProjectedBlock {
	hist := append([][2]float64(nil), m.Histogram...) // local copy; buckets are consumed as we go

	var out []domain.ProjectedBlock
	i := 0
	for b := 0; b < n && i < len(hist); b++ {
		var acc, totalFee float64
		var bucket [][2]float64
		minRate, maxRate := hist[i][0], hist[i][0]

		for i < len(hist) && acc < blockVSize {
			feerate, vsize := hist[i][0], hist[i][1]
			take := vsize
			if acc+take > blockVSize {
				take = blockVSize - acc
			}
			bucket = append(bucket, [2]float64{feerate, take})
			totalFee += feerate * take
			acc += take
			if feerate < minRate {
				minRate = feerate
			}
			if feerate > maxRate {
				maxRate = feerate
			}
			if take < vsize {
				hist[i][1] -= take // partially consumed; remainder rolls into the next block
				break
			}
			i++
		}

		out = append(out, domain.ProjectedBlock{
			VSize:         int64(acc),
			TotalFeeSats:  int64(totalFee),
			MedianFeeRate: weightedMedianRate(bucket),
			MinFeeRate:    minRate,
			MaxFeeRate:    maxRate,
			Approx:        true,
		})
	}
	return out
}

// weightedMedianRate returns the feerate at the vsize-weighted midpoint of
// buckets ordered descending by feerate.
func weightedMedianRate(buckets [][2]float64) float64 {
	if len(buckets) == 0 {
		return 0
	}
	var total float64
	for _, b := range buckets {
		total += b[1]
	}
	half := total / 2
	var acc float64
	for _, b := range buckets {
		acc += b[1]
		if acc >= half {
			return b[0]
		}
	}
	return buckets[len(buckets)-1][0]
}
