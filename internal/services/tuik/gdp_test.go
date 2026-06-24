package tuik

import "testing"

func TestAggregateGDPUsesProvinceWeightedPerCapita(t *testing.T) {
	gdp := CIPResponse{
		Dates: []string{"2024", "2023"},
		Rows: []CIPRow{
			{RegionCode: "1", Values: []string{"1000", "800"}},
			{RegionCode: "2", Values: []string{"2000", "1000"}},
		},
	}
	perCapTRY := CIPResponse{Rows: []CIPRow{
		{RegionCode: "1", Values: []string{"100", "100"}},
		{RegionCode: "2", Values: []string{"200", "100"}},
	}}
	perCapUSD := CIPResponse{Rows: []CIPRow{
		{RegionCode: "1", Values: []string{"10", "10"}},
		{RegionCode: "2", Values: []string{"20", "10"}},
	}}

	points, err := aggregateGDP(perCapUSD, perCapTRY, gdp)
	if err != nil {
		t.Fatalf("aggregateGDP: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("points = %d", len(points))
	}
	if points[0].GDPThousandTRY != 3000 {
		t.Fatalf("gdp = %.2f", points[0].GDPThousandTRY)
	}
	if points[0].PerCapitaTRY != 150 {
		t.Fatalf("per capita TRY = %.2f", points[0].PerCapitaTRY)
	}
	if points[0].PerCapitaUSD != 15 {
		t.Fatalf("per capita USD = %.2f", points[0].PerCapitaUSD)
	}
}
