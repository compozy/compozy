package gateway

func surfaceGenerationCurrent(snapshot Snapshot, expected SurfaceExposure) bool {
	for _, surface := range snapshot.Surfaces {
		if surface.Surface == expected.Surface && surface.Tier == expected.Tier {
			return surface.Generation == expected.Generation && surface.Desired == DesiredEnabled
		}
	}
	return false
}

func surfaceNames(exposures []SurfaceExposure) []Surface {
	result := make([]Surface, 0, len(exposures))
	for _, exposure := range exposures {
		result = append(result, exposure.Surface)
	}
	return result
}
