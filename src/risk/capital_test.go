package risk

import (
	"math"
	"testing"
)

func TestAssessPortfolioAppliesStressAndConcentration(t *testing.T) {
	inputs := []SegregationInput{
		{ID: "seg-eur", CustodyLiability: 400_000, ReserveAssets: 520_000, PendingOutflows: 40_000, LiquidReserve: 120_000, ReserveHaircutPPM: 20_000, OutflowShockPPM: 250_000, OperationalCostPPM: 5_000, MaturityEpochs: 3},
		{ID: "seg-usd", CustodyLiability: 600_000, ReserveAssets: 800_000, PendingOutflows: 50_000, LiquidReserve: 200_000, ReserveHaircutPPM: 10_000, OutflowShockPPM: 200_000, OperationalCostPPM: 5_000, MaturityEpochs: 7},
	}
	policy := CapitalPolicy{MinimumCoveragePPM: 1_050_000, MinimumLiquidityPPM: 1_000_000, MaximumHHIPPM: 550_000}

	assessment, err := AssessPortfolio(inputs, policy)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.TotalLiability != 1_000_000 {
		t.Fatalf("unexpected liability: %d", assessment.TotalLiability)
	}
	if assessment.TotalEffectiveReserve != 1_301_600 {
		t.Fatalf("unexpected effective reserve: %d", assessment.TotalEffectiveReserve)
	}
	if assessment.HHIPPM != 520_000 || assessment.WeightedMaturityEpochs != 5 {
		t.Fatalf("unexpected portfolio shape: hhi=%d maturity=%d", assessment.HHIPPM, assessment.WeightedMaturityEpochs)
	}
	if assessment.LargestConcentrationPPM != 600_000 {
		t.Fatalf("unexpected concentration: %d", assessment.LargestConcentrationPPM)
	}
	if !assessment.Compliant {
		t.Fatalf("expected compliant portfolio: %#v", assessment)
	}
	if assessment.Segregations[0].ID != "seg-eur" || assessment.Segregations[1].ID != "seg-usd" {
		t.Fatal("segregations are not deterministically ordered")
	}
}

func TestAssessPortfolioReportsCapitalDeficit(t *testing.T) {
	assessment, err := AssessPortfolio([]SegregationInput{{
		ID: "seg-usd", CustodyLiability: 1_000, ReserveAssets: 900, PendingOutflows: 100,
		LiquidReserve: 50, ReserveHaircutPPM: 100_000, OutflowShockPPM: 500_000,
		OperationalCostPPM: 10_000, MaturityEpochs: 2,
	}}, CapitalPolicy{MinimumCoveragePPM: PPM, MinimumLiquidityPPM: PPM, MaximumHHIPPM: PPM})
	if err != nil {
		t.Fatal(err)
	}
	line := assessment.Segregations[0]
	if line.EffectiveReserve != 810 || line.StressedOutflows != 150 || line.OperationalBuffer != 10 {
		t.Fatalf("unexpected stress line: %#v", line)
	}
	if line.CapitalDeficit != 350 || assessment.Compliant {
		t.Fatalf("unexpected deficit outcome: %#v", assessment)
	}
}

func TestAssessPortfolioRejectsInvalidSegregations(t *testing.T) {
	policy := CapitalPolicy{MinimumCoveragePPM: PPM, MinimumLiquidityPPM: PPM, MaximumHHIPPM: PPM}
	cases := [][]SegregationInput{
		{{ID: "", CustodyLiability: 1}},
		{{ID: "seg", CustodyLiability: 1, LiquidReserve: 2, ReserveAssets: 1}},
		{{ID: "seg", CustodyLiability: 1}, {ID: "seg", CustodyLiability: 1}},
		{{ID: "seg", CustodyLiability: 1, ReserveHaircutPPM: PPM + 1}},
	}
	for index, inputs := range cases {
		if _, err := AssessPortfolio(inputs, policy); err == nil {
			t.Fatalf("case %d should fail", index)
		}
	}
}

func TestAssessPortfolioUsesOverflowSafeMultiplication(t *testing.T) {
	assessment, err := AssessPortfolio([]SegregationInput{{
		ID: "large", CustodyLiability: math.MaxUint64 / 4, ReserveAssets: math.MaxUint64 / 3,
		PendingOutflows: 1, LiquidReserve: 1, ReserveHaircutPPM: 1, MaturityEpochs: 1,
	}}, CapitalPolicy{MinimumCoveragePPM: 1, MinimumLiquidityPPM: 1, MaximumHHIPPM: PPM})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.CoveragePPM == 0 {
		t.Fatal("expected a non-zero coverage ratio")
	}
}
