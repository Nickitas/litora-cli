package cli

import (
	"fmt"
	"strings"
)

func runCompletionCommand(app *App) error {
	shell := app.Config.CompletionShell
	if shell == "" {
		return fmt.Errorf("specify shell type: --shell=bash or --shell=zsh")
	}

	switch shell {
	case "bash":
		return generateBashCompletion()
	case "zsh":
		return generateZshCompletion()
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh)", shell)
	}
}

func generateBashCompletion() error {
	fmt.Println(`# bash completion for lito

_lito_completion() {
	local cur prev words cword
	_init_completion || return

	COMPREPLY=()

	case "${COMP_WORDS[1]}" in
		source)
			_lito_source_completion
			;;
		coastline)
			_lito_coastline_completion
			;;
		dimension)
			_lito_dimension_completion
			;;
		erosion)
			_lito_erosion_completion
			;;
		all)
			_lito_all_completion
			;;
		benchmark)
			_lito_benchmark_completion
			;;
		*)
			_lito_root_completion
			;;
	esac
}

_lito_root_completion() {
	local commands="source coastline dimension erosion all benchmark completion help"
	COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
}

_lito_source_completion() {
	case "${prev}" in
		--input)
			_filedir json
			return
			;;
		--source-url)
			# URL completion - skip
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_coastline_completion() {
	case "${prev}" in
		--input)
			_filedir json
			return
			;;
		--source-url)
			return
			;;
		--bathymetry)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --bathymetry --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_dimension_completion() {
	case "${prev}" in
		--input)
			_filedir json
			return
			;;
		--source-url)
			return
			;;
		--bathymetry)
			_filedir json
			return
			;;
		--lithology)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --iterations --seed --angle-jitter --height-jitter --bathymetry --lithology --enable-lithology --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_erosion_completion() {
	case "${prev}" in
		--input)
			_filedir json
			return
			;;
		--source-url)
			return
			;;
		--bathymetry)
			_filedir json
			return
			;;
		--lithology)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
		--output-csv)
			_filedir csv
			return
			;;
		--output-gif)
			_filedir gif
			return
			;;
	esac

	local flags="--input --source-url --refresh --steps --erosion-strength --wave-direction --wind-speed --fetch-spread --fetch-samples --max-fetch-km --depth-scale --exposure-power --bathymetry --lithology --enable-lithology --seed --target-years --years-per-step --storm-probability --storm-intensity --sea-level-rise --enable-seasonality --seasonal-phase --output --output-csv --csv-format --output-gif --gif-fps --gif-skip --gif-color-by-change --gif-show-initial --gif-show-metrics --gif-show-scale-bar --gif-show-color-legend --gif-scale-bar-km --gif-color-legend-pos --gif-geo-labels --gif-show-time-stamp --gif-width --gif-height --gif-colors --gif-compression --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_all_completion() {
	case "${prev}" in
		--input)
			_filedir json
			return
			;;
		--source-url)
			return
			;;
		--bathymetry)
			_filedir json
			return
			;;
		--lithology)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --iterations --seed --angle-jitter --height-jitter --steps --erosion-strength --wave-direction --wind-speed --fetch-spread --fetch-samples --max-fetch-km --depth-scale --exposure-power --bathymetry --lithology --enable-lithology --target-years --years-per-step --storm-probability --storm-intensity --sea-level-rise --enable-seasonality --seasonal-phase --model-max-points --no-model-simplify --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_benchmark_completion() {
	local subcommands="list init show calibrate calibrate-all analyze hotspots scenarios extract create"

	# After "benchmark" but before any subcommand
	if [[ ${COMP_WORDS[*]} =~ "benchmark" ]] && [[ ${#COMP_WORDS[@]} -eq 2 ]]; then
		COMPREPLY=($(compgen -W "${subcommands}" -- "${cur}"))
		return
	fi

	# After subcommand - complete subcommand-specific flags
	case "${COMP_WORDS[2]}" in
		show|calibrate|analyze|hotspots|scenarios)
			case "${prev}" in
				--site)
					_lito_complete_site_ids
					return
					;;
				--bathymetry)
					_filedir json
					return
					;;
				--output)
					_filedir json
					return
					;;
			esac
			local flags="--site --bathymetry --output --spectrum-spread --erosion-strength --wave-direction"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		calibrate-all)
			case "${prev}" in
				--bathymetry)
					_filedir json
					return
					;;
				--output)
					_filedir -d
					return
					;;
			esac
			local flags="--bathymetry --output --spectrum-spread"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		extract)
			local flags="--input --output --bounds-min-lat --bounds-max-lat --bounds-min-lon --bounds-max-lon"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		create)
			local flags="--name --region --country --description --bounds --coast-type --quality --wave-height --wave-period --wave-direction --lithology --years --coastline --dir"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		list|init)
			local flags="--dir"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
	esac
}

_lito_complete_site_ids() {
	local bench_dir="${LITO_BENCHMARK_DIR:-data/benchmarks}"
	if [[ -d "${bench_dir}" ]]; then
		local sites=($(cd "${bench_dir}" 2>/dev/null && ls *.json 2>/dev/null | sed 's/\.json$//'))
		COMPREPLY=($(compgen -W "${sites[*]}" -- "${cur}"))
	fi
}

complete -F _lito_completion lito`)
	return nil
}

func generateZshCompletion() error {
	fmt.Print(`#compdef lito

_lito() {
	local -a commands
	local -a benchmark_subcommands

	commands=(
		'source:Загрузка источника береговой линии'
		'coastline:Анализ реальных данных береговой линии'
		'dimension:Научная модель фрактальной размерности'
		'erosion:Модель волновой эрозии'
		'all:Полный сценарий проверки и моделирования'
		'benchmark:Калибровка и верификация модели'
		'completion:Генерация скриптов автодополнения'
		'help:Справка'
	)

	benchmark_subcommands=(
		'list:Список всех контрольных участков'
		'init:Инициализация стандартных участков Чёрного моря'
		'show:Подробная информация об участке'
		'calibrate:Калибровка параметров модели'
		'calibrate-all:Калибровка всех участков'
		'analyze:Полный статистический анализ'
		'hotspots:Поиск участков максимальной эрозии'
		'scenarios:Анализ климатических сценариев'
		'extract:Извлечение сегмента береговой линии'
		'create:Создание нового участка'
	)

	local curcontext="${curcontext}"
	local -a args

	case $service in
		_lito_command)
			_lito_commands() {
				_describe 'command' commands
			}

			_lito_benchmark() {
				_describe 'benchmark subcommand' benchmark_subcommands
			}

			args=(
				{-q,--quiet}'[Suppress startup banner]'
				'*: :->command'
			)

			_arguments -s $args && case $state in
				command)
					case $words[2] in
						benchmark)
							_lito_benchmark
							;;
						*)
							_lito_commands
							;;
					esac
					;;
			esac
			;;

		_lito_source)
			_arguments -S \
				'--input[Path to local coastline JSON/GeoJSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL for coastline data]:url:' \
				'(--refresh)--refresh[Force refresh of remote GeoJSON cache]' \
				'--output[Snapshot file or directory]:dir:_directories' \
				{-q,--quiet}'[Suppress startup banner]'
			;;

		_lito_coastline)
			_arguments -S \
				'--input[Path to local coastline JSON/GeoJSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL for coastline data]:url:' \
				'(--refresh)--refresh[Force refresh of remote GeoJSON cache]' \
				'--bathymetry[Path to bathymetry JSON file]:file:_files -g "*.json"' \
				'--output[Output file or directory]:dir:_directories' \
				{-q,--quiet}'[Suppress startup banner]'
			;;

		_lito_dimension)
			_arguments -S \
				'--input[Path to local coastline JSON/GeoJSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL for coastline data]:url:' \
				'(--refresh)--refresh[Force refresh of remote GeoJSON cache]' \
				'--iterations[Maximum organic Koch iterations]:iterations:(0 1 2 3 4 5 6 7)' \
				'--seed[Random seed for organic coastline]:seed:' \
				'--angle-jitter[Maximum random angle deviation in degrees]:jitter:(0 9 18 27 36)' \
				'--height-jitter[Maximum random height deviation as ratio]:jitter:(0 0.1 0.25 0.5)' \
				'--bathymetry[Path to bathymetry JSON file]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON file]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology-based erosion modulation]' \
				'--output[Output directory]:dir:_directories' \
				{-q,--quiet}'[Suppress startup banner]'
			;;

		_lito_erosion)
			_arguments -S \
				'--input[Path to local coastline JSON/GeoJSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL for coastline data]:url:' \
				'(--refresh)--refresh[Force refresh of remote GeoJSON cache]' \
				'--steps[Number of wave erosion steps]:steps:(1 5 10 20 50)' \
				'--erosion-strength[Gaussian erosion strength in meters]:strength:(0 5 10 20 30 50)' \
				'--wave-direction[Direction waves come from, degrees from N]:direction:(0 45 90 135 180 225 270 315)' \
				'--wind-speed[Wind speed driving wave energy, m/s]:speed:(5 10 12 15 20)' \
				'--bathymetry[Path to bathymetry JSON file]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON file]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology-based erosion modulation]' \
				'--target-years[Target simulation duration in years]:years:(10 20 50 100)' \
				'--years-per-step[Years per erosion step]:years:(0.5 1.0 2.0 5.0)' \
				'--storm-probability[Probability of storm event per step]:probability:(0 0.1 0.2 0.3)' \
				'--storm-intensity[Storm intensity multiplier]:intensity:(1.5 2.0 3.0 5.0)' \
				'--sea-level-rise[Sea level rise in meters per year]:rise:(0 0.001 0.005 0.01)' \
				'(--enable-seasonality)--enable-seasonality[Enable seasonal erosion variations]' \
				'--seasonal-phase[Seasonal phase offset in radians]:phase:(0 1.57 3.14)' \
				'--output[Output directory]:dir:_directories' \
				'--output-csv[Path to CSV file for metrics export]:file:_files' \
				'--csv-format[CSV format]:format:(long wide)' \
				'--output-gif[Path to GIF file for animation]:file:_files -g "*.gif"' \
				'--gif-fps[GIF frames per second]:fps:(5 10 15 20 30)' \
				'--gif-sip[Skip every N frames]:skip:(1 2 3 5)' \
				{-q,--quiet}'[Suppress startup banner]'
			;;

		_lito_all)
			_arguments -S \
				'--input[Path to local coastline JSON/GeoJSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL for coastline data]:url:' \
				'(--refresh)--refresh[Force refresh of remote GeoJSON cache]' \
				'--iterations[Maximum organic Koch iterations]:iterations:(0 1 2 3 4 5 6 7)' \
				'--seed[Random seed for organic coastline]:seed:' \
				'--angle-jitter[Maximum random angle deviation in degrees]:jitter:(0 9 18 27 36)' \
				'--height-jitter[Maximum random height deviation as ratio]:jitter:(0 0.1 0.25 0.5)' \
				'--steps[Number of wave erosion steps]:steps:(1 5 10 20 50)' \
				'--erosion-strength[Gaussian erosion strength in meters]:strength:(0 5 10 20 30 50)' \
				'--wave-direction[Direction waves come from, degrees from N]:direction:(0 45 90 135 180 225 270 315)' \
				'--wind-speed[Wind speed driving wave energy, m/s]:speed:(5 10 12 15 20)' \
				'--bathymetry[Path to bathymetry JSON file]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON file]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology-based erosion modulation]' \
				'--target-years[Target simulation duration in years]:years:(10 20 50 100)' \
				'--years-per-step[Years per erosion step]:years:(0.5 1.0 2.0 5.0)' \
				'--storm-probability[Probability of storm event per step]:probability:(0 0.1 0.2 0.3)' \
				'--storm-intensity[Storm intensity multiplier]:intensity:(1.5 2.0 3.0 5.0)' \
				'--sea-level-rise[Sea level rise in meters per year]:rise:(0 0.001 0.005 0.01)' \
				'(--enable-seasonality)--enable-seasonality[Enable seasonal erosion variations]' \
				'--output[Output directory]:dir:_directories' \
				{-q,--quiet}'[Suppress startup banner]'
			;;

		_lito_benchmark)
			if (( CURRENT == 2 )); then
				_describe 'benchmark subcommand' benchmark_subcommands
				return
			fi

			case $words[2] in
				list|init)
					_arguments '--dir[Directory for benchmark sites]:dir:_directories'
					;;
				show|calibrate|analyze|hotspots|scenarios)
					_arguments \
						'--site[Site ID for analysis]:site:->sites' \
						'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
						'--output[Output file path]:file:_files' \
						'--spectrum-spread[Wave spectrum spread in degrees]:spread:(0 15 30 45 60)' \
						'--erosion-strength[Erosion strength for hotspots]:strength:(5 10 15 20 30)' \
						'--wave-direction[Wave direction for hotspots]:direction:(0 45 90 135 180 225 270 315)'
					_lito_complete_sites
					;;
				calibrate-all)
					_arguments \
						'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
						'--output[Output directory]:dir:_directories' \
						'--spectrum-spread[Wave spectrum spread in degrees]:spread:(0 15 30 45 60)'
					;;
				extract)
					_arguments \
						'--input[Path to coastline JSON]:file:_files -g "*.json"' \
						'--output[Output file path]:file:_files' \
						'--bounds-min-lat[Minimum latitude]:lat:' \
						'--bounds-max-lat[Maximum latitude]:lat:' \
						'--bounds-min-lon[Minimum longitude]:lon:' \
						'--bounds-max-lon[Maximum longitude]:lon:'
					;;
				create)
					_arguments \
						'--name[Site name]:name:' \
						'--region[Region]:region:' \
						'--country[Country]:country:' \
						'--description[Description]:description:' \
						'--bounds[Bounds in format: min_lat,max_lat,min_lon,max_lon]:bounds:' \
						'--coast-type[Type of coastline]:type:(sandy cliff rocky muddy mixed artificial)' \
						'--quality[Data quality rating]:quality:(high medium low)' \
						'--wave-height[Mean wave height in meters]:height:' \
						'--wave-period[Mean wave period in seconds]:period:' \
						'--wave-direction[Mean wave direction in degrees]:direction:' \
						'--lithology[Dominant lithology]:lithology:' \
						'--years[Observation year range]:years:' \
						'--coastline[Path to coastline GeoJSON]:file:_files -g "*.geojson"' \
						'--dir[Benchmark directory]:dir:_directories'
					;;
			esac
			;;

		_lito_completion)
			_arguments '--shell[Shell type]:shell:(bash zsh)'
			;;
	esac
}

_lito_complete_sites() {
	local bench_dir="${LITO_BENCHMARK_DIR:-data/benchmarks}"
	if [[ -d "$bench_dir" ]]; then
		local -a sites
		sites=($bench_dir/*.json(N:t:r))
		if (( ${#sites} > 0 )); then
			compadd -a sites
		fi
	fi
}

_lito "$@"
`)
	return nil
}

// GetCompletionScript returns a completion script for the specified shell
func GetCompletionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return getBashCompletionScript(), nil
	case "zsh":
		return getZshCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

func getBashCompletionScript() string {
	return `# bash completion for lito
# Installation:
#   1. Source this file in ~/.bashrc or ~/.bash_profile:
#      source <(lito completion --shell=bash)
#   2. Or save to file and source it:
#      lito completion --shell=bash > /usr/local/share/bash-completion/completions/lito
#      source /usr/local/share/bash-completion/completions/lito

_lito_completion() {
	local cur prev words cword
	_init_completion || return

	COMPREPLY=()

	case "${COMP_WORDS[1]}" in
		source)
			_lito_source_completion
			;;
		coastline)
			_lito_coastline_completion
			;;
		dimension)
			_lito_dimension_completion
			;;
		erosion)
			_lito_erosion_completion
			;;
		all)
			_lito_all_completion
			;;
		benchmark)
			_lito_benchmark_completion
			;;
		completion)
			COMPREPLY=($(compgen -W "bash zsh" -- "${cur}"))
			;;
		help)
			COMPREPLY=($(compgen -W "source coastline dimension erosion all benchmark" -- "${cur}"))
			;;
		*)
			_lito_root_completion
			;;
	esac
}

_lito_root_completion() {
	local commands="source coastline dimension erosion all benchmark completion help"
	COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
}

_lito_source_completion() {
	case "${prev}" in
		--input|--source-url)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_coastline_completion() {
	case "${prev}" in
		--input|--source-url|--bathymetry)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --bathymetry --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_dimension_completion() {
	case "${prev}" in
		--input|--source-url|--bathymetry|--lithology)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --iterations --seed --angle-jitter --height-jitter --bathymetry --lithology --enable-lithology --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_erosion_completion() {
	case "${prev}" in
		--input|--source-url|--bathymetry|--lithology)
			_filedir json
			return
			;;
		--output|--output-csv)
			_filedir -d
			return
			;;
		--output-gif)
			_filedir gif
			return
			;;
	esac

	local flags="--input --source-url --refresh --steps --erosion-strength --wave-direction --wind-speed --fetch-spread --fetch-samples --max-fetch-km --depth-scale --exposure-power --bathymetry --lithology --enable-lithology --seed --target-years --years-per-step --storm-probability --storm-intensity --sea-level-rise --enable-seasonality --seasonal-phase --output --output-csv --csv-format --output-gif --gif-fps --gif-skip --gif-color-by-change --gif-show-initial --gif-show-metrics --gif-show-scale-bar --gif-show-color-legend --gif-scale-bar-km --gif-color-legend-pos --gif-geo-labels --gif-show-time-stamp --gif-width --gif-height --gif-colors --gif-compression --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_all_completion() {
	case "${prev}" in
		--input|--source-url|--bathymetry|--lithology)
			_filedir json
			return
			;;
		--output)
			_filedir -d
			return
			;;
	esac

	local flags="--input --source-url --refresh --iterations --seed --angle-jitter --height-jitter --steps --erosion-strength --wave-direction --wind-speed --fetch-spread --fetch-samples --max-fetch-km --depth-scale --exposure-power --bathymetry --lithology --enable-lithology --target-years --years-per-step --storm-probability --storm-intensity --sea-level-rise --enable-seasonality --seasonal-phase --model-max-points --no-model-simplify --output --quiet"
	COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
}

_lito_benchmark_completion() {
	local subcommands="list init show calibrate calibrate-all analyze hotspots scenarios extract create"

	if [[ ${COMP_WORDS[*]} =~ "benchmark" ]] && [[ ${#COMP_WORDS[@]} -eq 2 ]]; then
		COMPREPLY=($(compgen -W "${subcommands}" -- "${cur}"))
		return
	fi

	case "${COMP_WORDS[2]}" in
		show|calibrate|analyze|hotspots|scenarios)
			case "${prev}" in
				--site)
					_lito_complete_site_ids
					return
					;;
				--bathymetry)
					_filedir json
					return
					;;
				--output)
					_filedir json
					return
					;;
			esac
			local flags="--site --bathymetry --output --spectrum-spread --erosion-strength --wave-direction"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		calibrate-all)
			case "${prev}" in
				--bathymetry)
					_filedir json
					return
					;;
				--output)
					_filedir -d
					return
					;;
			esac
			local flags="--bathymetry --output --spectrum-spread"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		extract)
			local flags="--input --output --bounds-min-lat --bounds-max-lat --bounds-min-lon --bounds-max-lon"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		create)
			local flags="--name --region --country --description --bounds --coast-type --quality --wave-height --wave-period --wave-direction --lithology --years --coastline --dir"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
		list|init)
			local flags="--dir"
			COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
			;;
	esac
}

_lito_complete_site_ids() {
	local bench_dir="${LITO_BENCHMARK_DIR:-data/benchmarks}"
	if [[ -d "${bench_dir}" ]]; then
		local sites
		sites=($(cd "${bench_dir}" 2>/dev/null && ls *.json 2>/dev/null | sed 's/\.json$//'))
		COMPREPLY=($(compgen -W "${sites[*]}" -- "${cur}"))
	fi
}

complete -F _lito_completion lito`
}

func getZshCompletionScript() string {
	return `#compdef lito
# Installation:
#   1. Add this to your ~/.zshrc:
#      autoload -U compinit && compinit
#      eval "$(lito completion --shell=zsh)"
#   2. Or save to file:
#      lito completion --shell=zsh > /usr/local/share/zsh/vendor-completions/_lito
#      source /usr/local/share/zsh/vendor-completions/_lito

_lito() {
	local -a commands
	local -a benchmark_subcommands

	commands=(
		'source:Load coastline source data'
		'coastline:Analyze real coastline data'
		'dimension:Fractal dimension analysis'
		'erosion:Wave erosion simulation'
		'all:Full validation and modeling scenario'
		'benchmark:Calibration and verification'
		'completion:Generate completion scripts'
		'help:Show help'
	)

	benchmark_subcommands=(
		'list:List all benchmark sites'
		'init:Initialize Black Sea benchmark sites'
		'show:Show site details'
		'calibrate:Calibrate model parameters'
		'calibrate-all:Calibrate all sites'
		'analyze:Full statistical analysis'
		'hotspots:Find erosion hotspots'
		'scenarios:Climate scenario analysis'
		'extract:Extract coastline segment'
		'create:Create new benchmark site'
	)

	local curcontext="${curcontext}"
	local -a args

	case $service in
		_lito_command)
			args=(
				{-q,--quiet}'[Suppress startup banner]'
				'*: :->cmds'
			)

			_arguments -s $args

			case $state in
				cmds)
					if (( CURRENT == 2 )); then
						_describe 'command' commands
					elif [[ $words[2] == "benchmark" ]] && (( CURRENT == 3 )); then
						_describe 'benchmark subcommand' benchmark_subcommands
					fi
					;;
			esac
			;;

		_lito_source)
			_arguments -S \
				'--input[Path to local coastline JSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL]:url:_urls' \
				'(--refresh)--refresh[Force refresh cache]' \
				'--output[Snapshot directory]:dir:_directories' \
				{-q,--quiet}'[Suppress banner]'
			;;

		_lito_coastline)
			_arguments -S \
				'--input[Path to local coastline JSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL]:url:_urls' \
				'(--refresh)--refresh[Force refresh cache]' \
				'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
				'--output[Output directory]:dir:_directories' \
				{-q,--quiet}'[Suppress banner]'
			;;

		_lito_dimension)
			_arguments -S \
				'--input[Path to local coastline JSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL]:url:_urls' \
				'(--refresh)--refresh[Force refresh cache]' \
				'--iterations[Max Koch iterations]:iterations:(0 1 2 3 4 5 6 7)' \
				'--seed[Random seed]:seed:' \
				'--angle-jitter[Max angle jitter deg]:jitter:(0 9 18 27 36)' \
				'--height-jitter[Max height jitter ratio]:jitter:(0 0.1 0.25 0.5)' \
				'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology]' \
				'--output[Output directory]:dir:_directories' \
				{-q,--quiet}'[Suppress banner]'
			;;

		_lito_erosion)
			_arguments -S \
				'--input[Path to local coastline JSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL]:url:_urls' \
				'(--refresh)--refresh[Force refresh cache]' \
				'--steps[Number of erosion steps]:steps:(1 5 10 20 50)' \
				'--erosion-strength[Erosion strength m]:strength:(0 5 10 20 30 50)' \
				'--wave-direction[Wave direction deg]:direction:(0 45 90 135 180 225 270 315)' \
				'--wind-speed[Wind speed m/s]:speed:(5 10 12 15 20)' \
				'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology]' \
				'--target-years[Target years]:years:(10 20 50 100)' \
				'--years-per-step[Years per step]:years:(0.5 1.0 2.0 5.0)' \
				'--storm-probability[Storm probability]:probability:(0 0.1 0.2 0.3)' \
				'--storm-intensity[Storm intensity]:intensity:(1.5 2.0 3.0 5.0)' \
				'--sea-level-rise[Sea level rise m/year]:rise:(0 0.001 0.005 0.01)' \
				'(--enable-seasonality)--enable-seasonality[Enable seasonality]' \
				'--seasonal-phase[Seasonal phase rad]:phase:(0 1.57 3.14)' \
				'--output[Output directory]:dir:_directories' \
				'--output-csv[CSV output path]:file:_files' \
				'--csv-format[CSV format]:format:(long wide)' \
				'--output-gif[GIF output path]:file:_files -g "*.gif"' \
				'--gif-fps[GIF FPS]:fps:(5 10 15 20 30)' \
				'--gif-skip[Skip frames]:skip:(1 2 3 5)' \
				{-q,--quiet}'[Suppress banner]'
			;;

		_lito_all)
			_arguments -S \
				'--input[Path to local coastline JSON]:file:_files -g "*.json"' \
				'--source-url[Remote GeoJSON URL]:url:_urls' \
				'(--refresh)--refresh[Force refresh cache]' \
				'--iterations[Max Koch iterations]:iterations:(0 1 2 3 4 5 6 7)' \
				'--seed[Random seed]:seed:' \
				'--angle-jitter[Max angle jitter deg]:jitter:(0 9 18 27 36)' \
				'--height-jitter[Max height jitter ratio]:jitter:(0 0.1 0.25 0.5)' \
				'--steps[Number of erosion steps]:steps:(1 5 10 20 50)' \
				'--erosion-strength[Erosion strength m]:strength:(0 5 10 20 30 50)' \
				'--wave-direction[Wave direction deg]:direction:(0 45 90 135 180 225 270 315)' \
				'--wind-speed[Wind speed m/s]:speed:(5 10 12 15 20)' \
				'--bathymetry[Path to bathymetry JSON]:file:_files -g "*.json"' \
				'--lithology[Path to lithology JSON]:file:_files -g "*.json"' \
				'(--enable-lithology)--enable-lithology[Enable lithology]' \
				'--target-years[Target years]:years:(10 20 50 100)' \
				'--years-per-step[Years per step]:years:(0.5 1.0 2.0 5.0)' \
				'--storm-probability[Storm probability]:probability:(0 0.1 0.2 0.3)' \
				'--storm-intensity[Storm intensity]:intensity:(1.5 2.0 3.0 5.0)' \
				'--sea-level-rise[Sea level rise m/year]:rise:(0 0.001 0.005 0.01)' \
				'(--enable-seasonality)--enable-seasonality[Enable seasonality]' \
				'--output[Output directory]:dir:_directories' \
				{-q,--quiet}'[Suppress banner]'
			;;

		_lito_benchmark)
			if (( CURRENT == 3 )); then
				_describe 'benchmark subcommand' benchmark_subcommands
				return
			fi

			case $words[3] in
				show|calibrate|analyze|hotspots|scenarios)
					_arguments \
						'--site[Site ID]:site:->sites' \
						'--bathymetry[Bathymetry JSON]:file:_files -g "*.json"' \
						'--output[Output file]:file:_files' \
						'--spectrum-spread[Spectrum spread deg]:spread:(0 15 30 45 60)' \
						'--erosion-strength[Strength m]:strength:(5 10 15 20 30)' \
						'--wave-direction[Wave direction]:direction:(0 45 90 135 180 225 270 315)'
					_lito_complete_sites
					;;
				calibrate-all)
					_arguments \
						'--bathymetry[Bathymetry JSON]:file:_files -g "*.json"' \
						'--output[Output directory]:dir:_directories' \
						'--spectrum-spread[Spectrum spread deg]:spread:(0 15 30 45 60)'
					;;
				extract)
					_arguments \
						'--input[Coastline JSON]:file:_files -g "*.json"' \
						'--output[Output file]:file:_files' \
						'--bounds-min-lat[Min lat]:lat:' \
						'--bounds-max-lat[Max lat]:lat:' \
						'--bounds-min-lon[Min lon]:lon:' \
						'--bounds-max-lon[Max lon]:lon:'
					;;
				create)
					_arguments \
						'--name[Site name]:name:' \
						'--region[Region]:region:' \
						'--country[Country]:country:' \
						'--description[Description]:description:' \
						'--bounds[Bounds format: min_lat,max_lat,min_lon,max_lon]:bounds:' \
						'--coast-type[Coast type]:type:(sandy cliff rocky muddy mixed artificial)' \
						'--quality[Data quality]:quality:(high medium low)' \
						'--wave-height[Wave height m]:height:' \
						'--wave-period[Wave period s]:period:' \
						'--wave-direction[Wave direction]:direction:' \
						'--lithology[Lithology]:lithology:' \
						'--years[Observation years]:years:' \
						'--coastline[Coastline GeoJSON]:file:_files -g "*.geojson"' \
						'--dir[Benchmark dir]:dir:_directories'
					;;
				list|init)
					_arguments '--dir[Benchmark directory]:dir:_directories'
					;;
			esac
			;;

		_lito_completion)
			_arguments '--shell[Shell type]:shell:(bash zsh)'
			;;
	esac
}

_lito_complete_sites() {
	local bench_dir="${LITO_BENCHMARK_DIR:-data/benchmarks}"
	if [[ -d "$bench_dir" ]]; then
		local -a sites
		sites=($bench_dir/*.json(N:t:r))
		if (( ${#sites} > 0 )); then
			compadd -a sites
		fi
	fi
}

_lito "$@"
`
}

// PrintInstallationInstructions prints installation instructions for completion
func PrintCompletionInstallationInstructions(shell string) {
	fmt.Printf("\nInstallation instructions for %s:\n\n", strings.ToUpper(shell))

	switch shell {
	case "bash":
		fmt.Println("  # Temporary (current session only):")
		fmt.Println("  source <(lito completion --shell=bash)")
		fmt.Println()
		fmt.Println("  # Permanent (add to ~/.bashrc or ~/.bash_profile):")
		fmt.Println("  eval \"$(lito completion --shell=bash)\"")
		fmt.Println()
		fmt.Println("  # Or install system-wide:")
		fmt.Println("  sudo mkdir -p /usr/local/share/bash-completion/completions")
		fmt.Println("  sudo lito completion --shell=bash > /usr/local/share/bash-completion/completions/lito")
		fmt.Println("  source /usr/local/share/bash-completion/completions/lito")
		fmt.Println()

	case "zsh":
		fmt.Println("  # Temporary (current session only):")
		fmt.Println("  autoload -U compinit && compinit")
		fmt.Println("  eval \"$(lito completion --shell=zsh)\"")
		fmt.Println()
		fmt.Println("  # Permanent (add to ~/.zshrc):")
		fmt.Println("  eval \"$(lito completion --shell=zsh)\"")
		fmt.Println()
		fmt.Println("  # Or install system-wide:")
		fmt.Println("  sudo mkdir -p /usr/local/share/zsh/vendor-completions")
		fmt.Println("  sudo lito completion --shell=zsh > /usr/local/share/zsh/vendor-completions/_lito")
		fmt.Println("  source /usr/local/share/zsh/vendor-completions/_lito")
		fmt.Println()
	}
}
