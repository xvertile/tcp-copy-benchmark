package main

import (
	"fmt"
	"github.com/google/pprof/profile"
	"os"
	"sort"
)

var profiles = []string{
	"cpu_profile_Bufio.prof",
	"cpu_profile_IoCopy.prof",
	"cpu_profile_IoCopyBuffer.prof",
	"cpu_profile_IoPipe.prof",
	"cpu_profile_IoCopyDirect.prof",
	"cpu_profile_ReadvWritev.prof",
	"cpu_profile_Sendfile.prof",
	"cpu_profile_Splice.prof",
	"cpu_profile_Syscall.prof",
	"cpu_profile_UnixSyscall.prof",
}

func main() {
	type ProfileInfo struct {
		Name      string
		CPUTime   int64
		CPUTimeMs float64
	}

	var profileInfos []ProfileInfo

	for _, profilePath := range profiles {
		f, err := os.Open("results/" + profilePath)
		if err != nil {
			fmt.Printf("Error opening profile %s: %v\n", profilePath, err)
			continue
		}
		defer f.Close()

		prof, err := profile.Parse(f)
		if err != nil {
			fmt.Printf("Error parsing profile %s: %v\n", profilePath, err)
			continue
		}

		var totalTime int64
		for _, sample := range prof.Sample {
			for _, value := range sample.Value {
				totalTime += value
			}
		}

		// Convert CPU time from nanoseconds to milliseconds
		totalTimeMs := float64(totalTime) / 1e6
		profileInfos = append(profileInfos, ProfileInfo{profilePath, totalTime, totalTimeMs})
	}

	// Sort profiles by CPU time
	sort.Slice(profileInfos, func(i, j int) bool {
		return profileInfos[i].CPUTime < profileInfos[j].CPUTime
	})

	// Print CPU times and top 3 profiles
	fmt.Println("CPU time (in ms) for each profile:")
	for _, info := range profileInfos {
		fmt.Printf("Profile %s: %.2f ms\n", info.Name, info.CPUTimeMs)
	}

	fmt.Println("\nTop 3 profiles with the least CPU usage:")
	for i := 0; i < 3 && i < len(profileInfos); i++ {
		fmt.Printf("%d. Profile %s: %.2f ms\n", i+1, profileInfos[i].Name, profileInfos[i].CPUTimeMs)
	}
}
