package risk

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
)

const PPM uint64 = 1_000_000

type SegregationInput struct {
	ID                 string `json:"id"`
	CustodyLiability   uint64 `json:"custody_liability"`
	ReserveAssets      uint64 `json:"reserve_assets"`
	PendingOutflows    uint64 `json:"pending_outflows"`
	LiquidReserve      uint64 `json:"liquid_reserve"`
	ReserveHaircutPPM  uint64 `json:"reserve_haircut_ppm"`
	OutflowShockPPM    uint64 `json:"outflow_shock_ppm"`
	OperationalCostPPM uint64 `json:"operational_cost_ppm"`
	MaturityEpochs     uint64 `json:"maturity_epochs"`
}

type SegregationAssessment struct {
	ID                    string `json:"id"`
	CustodyLiability      uint64 `json:"custody_liability"`
	EffectiveReserve      uint64 `json:"effective_reserve"`
	StressedOutflows      uint64 `json:"stressed_outflows"`
	OperationalBuffer     uint64 `json:"operational_buffer"`
	RequiredCapital       uint64 `json:"required_capital"`
	CapitalDeficit        uint64 `json:"capital_deficit"`
	CoveragePPM           uint64 `json:"coverage_ppm"`
	LiquidityPPM          uint64 `json:"liquidity_ppm"`
	MaturityEpochs        uint64 `json:"maturity_epochs"`
	ConcentrationSharePPM uint64 `json:"concentration_share_ppm"`
}

type CapitalPolicy struct {
	MinimumCoveragePPM  uint64 `json:"minimum_coverage_ppm"`
	MinimumLiquidityPPM uint64 `json:"minimum_liquidity_ppm"`
	MaximumHHIPPM       uint64 `json:"maximum_hhi_ppm"`
}

type PortfolioAssessment struct {
	TotalLiability          uint64                  `json:"total_liability"`
	TotalEffectiveReserve   uint64                  `json:"total_effective_reserve"`
	TotalRequiredCapital    uint64                  `json:"total_required_capital"`
	TotalCapitalDeficit     uint64                  `json:"total_capital_deficit"`
	CoveragePPM             uint64                  `json:"coverage_ppm"`
	LiquidityPPM            uint64                  `json:"liquidity_ppm"`
	LargestConcentrationPPM uint64                  `json:"largest_concentration_ppm"`
	HHIPPM                  uint64                  `json:"hhi_ppm"`
	WeightedMaturityEpochs  uint64                  `json:"weighted_maturity_epochs"`
	Compliant               bool                    `json:"compliant"`
	Segregations            []SegregationAssessment `json:"segregations"`
}

func AssessPortfolio(inputs []SegregationInput, policy CapitalPolicy) (PortfolioAssessment, error) {
	if len(inputs) == 0 {
		return PortfolioAssessment{}, fmt.Errorf("capital: at least one segregation is required")
	}
	if policy.MinimumCoveragePPM == 0 || policy.MinimumLiquidityPPM == 0 {
		return PortfolioAssessment{}, fmt.Errorf("capital: coverage and liquidity thresholds must be positive")
	}
	if policy.MaximumHHIPPM == 0 || policy.MaximumHHIPPM > PPM {
		return PortfolioAssessment{}, fmt.Errorf("capital: HHI threshold must be within (0, %d]", PPM)
	}

	ordered := append([]SegregationInput(nil), inputs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	seen := make(map[string]struct{}, len(ordered))
	result := PortfolioAssessment{Segregations: make([]SegregationAssessment, 0, len(ordered))}
	var totalLiquid uint64
	var totalStressedOutflows uint64

	for _, input := range ordered {
		if err := validateInput(input, seen); err != nil {
			return PortfolioAssessment{}, err
		}
		seen[input.ID] = struct{}{}
		assessment, err := assessSegregation(input)
		if err != nil {
			return PortfolioAssessment{}, fmt.Errorf("capital: segregation %s: %w", input.ID, err)
		}
		result.Segregations = append(result.Segregations, assessment)
		if result.TotalLiability, err = add(result.TotalLiability, assessment.CustodyLiability); err != nil {
			return PortfolioAssessment{}, err
		}
		if result.TotalEffectiveReserve, err = add(result.TotalEffectiveReserve, assessment.EffectiveReserve); err != nil {
			return PortfolioAssessment{}, err
		}
		if result.TotalRequiredCapital, err = add(result.TotalRequiredCapital, assessment.RequiredCapital); err != nil {
			return PortfolioAssessment{}, err
		}
		if result.TotalCapitalDeficit, err = add(result.TotalCapitalDeficit, assessment.CapitalDeficit); err != nil {
			return PortfolioAssessment{}, err
		}
		if totalLiquid, err = add(totalLiquid, input.LiquidReserve); err != nil {
			return PortfolioAssessment{}, err
		}
		if totalStressedOutflows, err = add(totalStressedOutflows, assessment.StressedOutflows); err != nil {
			return PortfolioAssessment{}, err
		}
	}

	var err error
	result.CoveragePPM, err = ratioPPM(result.TotalEffectiveReserve, result.TotalRequiredCapital)
	if err != nil {
		return PortfolioAssessment{}, err
	}
	result.LiquidityPPM, err = ratioPPM(totalLiquid, totalStressedOutflows)
	if err != nil {
		return PortfolioAssessment{}, err
	}
	result.HHIPPM, result.WeightedMaturityEpochs, err = portfolioShape(ordered, result.TotalLiability)
	if err != nil {
		return PortfolioAssessment{}, err
	}
	for i := range result.Segregations {
		share, shareErr := ratioPPM(result.Segregations[i].CustodyLiability, result.TotalLiability)
		if shareErr != nil {
			return PortfolioAssessment{}, shareErr
		}
		result.Segregations[i].ConcentrationSharePPM = share
		if share > result.LargestConcentrationPPM {
			result.LargestConcentrationPPM = share
		}
	}
	result.Compliant = result.TotalCapitalDeficit == 0 &&
		result.CoveragePPM >= policy.MinimumCoveragePPM &&
		result.LiquidityPPM >= policy.MinimumLiquidityPPM &&
		result.HHIPPM <= policy.MaximumHHIPPM
	return result, nil
}

func validateInput(input SegregationInput, seen map[string]struct{}) error {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return fmt.Errorf("capital: segregation id is required")
	}
	if id != input.ID {
		return fmt.Errorf("capital: segregation id %q contains surrounding whitespace", input.ID)
	}
	if _, exists := seen[id]; exists {
		return fmt.Errorf("capital: duplicate segregation id %q", id)
	}
	if input.CustodyLiability == 0 {
		return fmt.Errorf("capital: segregation %s has zero custody liability", id)
	}
	if input.LiquidReserve > input.ReserveAssets {
		return fmt.Errorf("capital: segregation %s has liquid reserve above total reserve", id)
	}
	if input.ReserveHaircutPPM > PPM {
		return fmt.Errorf("capital: segregation %s haircut exceeds %d ppm", id, PPM)
	}
	return nil
}

func assessSegregation(input SegregationInput) (SegregationAssessment, error) {
	effectiveReserve, err := mulDivFloor(input.ReserveAssets, PPM-input.ReserveHaircutPPM, PPM)
	if err != nil {
		return SegregationAssessment{}, err
	}
	stressedOutflows, err := mulDivCeil(input.PendingOutflows, PPM+input.OutflowShockPPM, PPM)
	if err != nil {
		return SegregationAssessment{}, err
	}
	operationalBuffer, err := mulDivCeil(input.CustodyLiability, input.OperationalCostPPM, PPM)
	if err != nil {
		return SegregationAssessment{}, err
	}
	required, err := add(input.CustodyLiability, stressedOutflows)
	if err != nil {
		return SegregationAssessment{}, err
	}
	required, err = add(required, operationalBuffer)
	if err != nil {
		return SegregationAssessment{}, err
	}
	deficit := uint64(0)
	if effectiveReserve < required {
		deficit = required - effectiveReserve
	}
	coverage, err := ratioPPM(effectiveReserve, required)
	if err != nil {
		return SegregationAssessment{}, err
	}
	liquidity, err := ratioPPM(input.LiquidReserve, stressedOutflows)
	if err != nil {
		return SegregationAssessment{}, err
	}
	return SegregationAssessment{
		ID:                input.ID,
		CustodyLiability:  input.CustodyLiability,
		EffectiveReserve:  effectiveReserve,
		StressedOutflows:  stressedOutflows,
		OperationalBuffer: operationalBuffer,
		RequiredCapital:   required,
		CapitalDeficit:    deficit,
		CoveragePPM:       coverage,
		LiquidityPPM:      liquidity,
		MaturityEpochs:    input.MaturityEpochs,
	}, nil
}

func portfolioShape(inputs []SegregationInput, totalLiability uint64) (uint64, uint64, error) {
	if totalLiability == 0 {
		return 0, 0, fmt.Errorf("capital: total liability is zero")
	}
	total := new(big.Int).SetUint64(totalLiability)
	denominator := new(big.Int).Mul(new(big.Int).Set(total), total)
	squared := new(big.Int)
	weighted := new(big.Int)
	for _, input := range inputs {
		liability := new(big.Int).SetUint64(input.CustodyLiability)
		squared.Add(squared, new(big.Int).Mul(liability, liability))
		weighted.Add(weighted, new(big.Int).Mul(liability, new(big.Int).SetUint64(input.MaturityEpochs)))
	}
	hhi := new(big.Int).Quo(new(big.Int).Mul(squared, new(big.Int).SetUint64(PPM)), denominator)
	maturity := new(big.Int).Quo(weighted, total)
	if !hhi.IsUint64() || !maturity.IsUint64() {
		return 0, 0, fmt.Errorf("capital: portfolio shape exceeds uint64")
	}
	return hhi.Uint64(), maturity.Uint64(), nil
}

func ratioPPM(numerator uint64, denominator uint64) (uint64, error) {
	if denominator == 0 {
		if numerator == 0 {
			return 0, nil
		}
		return math.MaxUint64, nil
	}
	return mulDivFloor(numerator, PPM, denominator)
}

func mulDivFloor(a uint64, b uint64, denominator uint64) (uint64, error) {
	if denominator == 0 {
		return 0, fmt.Errorf("capital: zero denominator")
	}
	product := new(big.Int).Mul(new(big.Int).SetUint64(a), new(big.Int).SetUint64(b))
	quotient := new(big.Int).Quo(product, new(big.Int).SetUint64(denominator))
	if !quotient.IsUint64() {
		return 0, fmt.Errorf("capital: arithmetic result exceeds uint64")
	}
	return quotient.Uint64(), nil
}

func mulDivCeil(a uint64, b uint64, denominator uint64) (uint64, error) {
	if denominator == 0 {
		return 0, fmt.Errorf("capital: zero denominator")
	}
	product := new(big.Int).Mul(new(big.Int).SetUint64(a), new(big.Int).SetUint64(b))
	if product.Sign() == 0 {
		return 0, nil
	}
	product.Add(product, new(big.Int).Sub(new(big.Int).SetUint64(denominator), big.NewInt(1)))
	quotient := new(big.Int).Quo(product, new(big.Int).SetUint64(denominator))
	if !quotient.IsUint64() {
		return 0, fmt.Errorf("capital: arithmetic result exceeds uint64")
	}
	return quotient.Uint64(), nil
}

func add(a uint64, b uint64) (uint64, error) {
	if math.MaxUint64-a < b {
		return 0, fmt.Errorf("capital: arithmetic addition exceeds uint64")
	}
	return a + b, nil
}
