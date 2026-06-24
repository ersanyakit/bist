package validation

import "time"

type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type Split struct {
	Train      DateRange `json:"train"`
	Validation DateRange `json:"validation"`
	Test       DateRange `json:"test"`
}

func ChronologicalSplit(dates []time.Time, trainRatio, validationRatio float64) Split {
	if len(dates) == 0 {
		return Split{}
	}
	if trainRatio <= 0 || trainRatio >= 1 {
		trainRatio = 0.60
	}
	if validationRatio <= 0 || trainRatio+validationRatio >= 1 {
		validationRatio = 0.20
	}
	trainEnd := int(float64(len(dates)) * trainRatio)
	valEnd := int(float64(len(dates)) * (trainRatio + validationRatio))
	if trainEnd < 1 {
		trainEnd = 1
	}
	if valEnd <= trainEnd {
		valEnd = trainEnd + 1
	}
	if valEnd >= len(dates) {
		valEnd = len(dates) - 1
	}
	return Split{
		Train:      DateRange{From: dates[0], To: dates[trainEnd-1]},
		Validation: DateRange{From: dates[trainEnd], To: dates[valEnd-1]},
		Test:       DateRange{From: dates[valEnd], To: dates[len(dates)-1]},
	}
}
