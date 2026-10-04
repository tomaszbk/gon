# Go / Gon benchmark results

Run: 20261004T013816Z; macOS-27.0.1-arm64-arm-64bit-Mach-O; Apple M4.

10 runtime samples per variant, 300ms per benchmark/sample; 7 build samples. Positive percentages mean slower.

Every operation processes 64 elements. All benchmarks are shown, including regressions.

| Workload | Go legacy ns/op | Gon legacy ns/op | Gon modern ns/op | Modern / Gon legacy | B/op (Go / Gon / modern) | Allocs/op (Go / Gon / modern) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| CoalesceAssignMixedBatch | 55.2 | 48.1 | 46.3 | -3.5% [-3.8%, -3.3%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ConditionalMixedBatch | 28.0 | 28.2 | 27.9 | -1.0% [-1.6%, -0.7%] | 0 / 0 / 0 | 0 / 0 / 0 |
| EnumConstructionBatch | 55.2 | 54.6 | 55.3 | +0.6% [-1.5%, +2.7%] | 0 / 0 / 0 | 0 / 0 / 0 |
| EnumCopyBatch | 50.7 | 50.7 | 50.7 | +0.1% [-0.2%, +6.7%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ErrorHandlerMixedBatch | 1,068.5 | 1,090.0 | 1,077.5 | -0.7% [-4.4%, +1.1%] | 1024 / 1024 / 1024 | 32 / 32 / 32 |
| ErrorPropagationFailureBatch | 53.2 | 53.2 | 53.3 | -0.1% [-0.7%, +0.4%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ErrorPropagationSuccessBatch | 88.7 | 80.6 | 78.2 | -3.8% [-12.2%, +8.1%] | 0 / 0 / 0 | 0 / 0 / 0 |
| LambdaCapturedBatch | 22.8 | 22.8 | 22.7 | -0.3% [-0.7%, +0.1%] | 0 / 0 / 0 | 0 / 0 / 0 |
| MatchingExhaustiveBatch | 126.4 | 125.7 | 115.2 | -8.5% [-9.3%, -8.0%] | 0 / 0 / 0 | 0 / 0 / 0 |
| MatchingGuardedBatch | 129.1 | 128.8 | 122.1 | -5.8% [-6.8%, -3.2%] | 0 / 0 / 0 | 0 / 0 / 0 |
| NamedReorderedBatch | 117.8 | 117.7 | 117.7 | +0.0% [-0.1%, +0.2%] | 0 / 0 / 0 | 0 / 0 / 0 |
| NamedVariadicBatch | 118.5 | 118.2 | 103.6 | -12.3% [-12.4%, -12.0%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionAbsentBatch | 40.6 | 40.5 | 41.0 | +1.3% [+1.1%, +1.8%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionAssignmentMixedBatch | 57.4 | 49.4 | 49.0 | -0.6% [-1.7%, -0.3%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionPresentBatch | 37.5 | 37.5 | 37.5 | -0.1% [-0.3%, +0.4%] | 0 / 0 / 0 | 0 / 0 / 0 |
| PipelineFailureBatch | 5,113.5 | 5,107.0 | 5,088.5 | -0.1% [-1.2%, +0.8%] | 6400 / 6400 / 6400 | 160 / 160 / 160 |
| PipelineMixedBatch | 2,774.5 | 2,750.5 | 2,767.5 | +0.6% [-0.9%, +1.8%] | 3200 / 3200 / 3200 | 80 / 80 / 80 |
| PipelineSuccessBatch | 372.4 | 370.5 | 373.8 | +0.7% [-0.8%, +2.6%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ResultFailureBatch | 37.6 | 37.2 | 37.2 | +0.1% [-0.1%, +0.2%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ResultHandlerMixedBatch | 54.8 | 54.4 | 54.5 | +0.5% [-1.4%, +1.6%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ResultSuccessBatch | 40.7 | 47.2 | 41.8 | -6.8% [-18.4%, +4.1%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeCallMixedBatch | 85.7 | 96.9 | 96.5 | -0.3% [-0.6%, -0.1%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeFieldAbsentBatch | 28.0 | 27.5 | 27.8 | +6.1% [-3.4%, +11.8%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeFieldPresentBatch | 25.3 | 25.3 | 25.3 | -0.0% [-0.8%, +0.2%] | 0 / 0 / 0 | 0 / 0 / 0 |

## Representation

Sizes are bytes per value on this host. Retention is one untimed process sample per variant, with 10,000 present pointer payloads across a forced GC; heap deltas include runtime/allocator noise.

| Shape | Go legacy bytes | Gon legacy bytes | Gon modern bytes |
| --- | ---: | ---: | ---: |
| (*int)? | 16 | 16 | 16 |
| Result[int,error] | 32 | 32 | 32 |
| Result[int,string] | 32 | 32 | 32 |
| enum/record | 32 | 32 | 32 |
| enum/unit | 8 | 8 | 16 |
| int? | 16 | 16 | 16 |
| struct{}? | 2 | 2 | 16 |

| Variant | Slice storage bytes | Observed heap delta bytes | GC cycles | Checksum |
| --- | ---: | ---: | ---: | ---: |
| go-legacy | 160000 | 243824 | 1 | 49995000 |
| gon-legacy | 160000 | 243824 | 1 | 49995000 |
| gon-modern | 160000 | 243824 | 1 | 49995000 |

## Build

| Variant | Rebuild median seconds | No-change median seconds | Rebuild CPU seconds | Reported RSS MiB | Executable bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| go-legacy | 0.243 | 0.108 | 0.180 | 85.4 | 3819218 |
| gon-legacy | 0.246 | 0.110 | 0.180 | 79.5 | 3735250 |
| gon-modern | 0.247 | 0.112 | 0.180 | 79.6 | 3735202 |

## Reproduction and limits

- go: `go version go1.27.1 darwin/arm64`; compiler SHA-256 `da10adfefec48f7681707909e213b24c423f5f5aa7efebfcede7eabb1c3b4156`.
- gon: `go version go1.28-devel_9c908bca8a Wed Sep 30 19:19:34 2026 -0300 darwin/arm64`; compiler SHA-256 `4706cb522612aaf27ecf0105d9d89c7cc53948b93de1594663623cc254227fc9`.
- Fixture `common.go` SHA-256: `de2d5516bc3297748de6fbfe8768e918eb8af97e67d8bb8e5022e17a22736a54`.
- Fixture `common_test.go` SHA-256: `48d3aacfb0953142a2eb383acd1f5c36e721805c8862d54c2a78a3d06b769c77`.
- Fixture `legacy.go` SHA-256: `70c376a6d8957cc9f972e33fa9fa802ee36f185cb29dc66bb8066d678e8dee96`.
- Fixture `modern.go` SHA-256: `0d15aab024a3a3dd9b7818f90608de8dbda1228ee21cc84ce20edd50147d3c42`.

- Go clásico y Gon clásico usan fuentes idénticas (SHA-256 comprobado). Las tres variantes comparten datos, pruebas y benchmarks.
- Go y Gon tienen versiones base distintas: esa comparación mezcla cambios upstream, del runtime y de Gon. Gon moderno vs Gon clásico aísla la sintaxis en el mismo toolchain.
- Ejecución serial, orden de variantes aleatorio en cada ronda, GOMAXPROCS=1, CGO desactivado, optimizaciones por defecto; no se desactiva inlining.
- Cada ns/op, B/op y allocs/op corresponde a un lote de 64 elementos. Preparación de datos fuera del cronómetro; sumas consumidas por sinks globales. Contadores compartidos hacen observable la evaluación y agregan costo de instrumentación.
- Las asignaciones son bytes/objetos de heap por operación, no memoria total del proceso. Los buffers reutilizados no cuentan como asignaciones nuevas.
- Representation: tamaños de valor por unsafe.Sizeof y una muestra no temporizada de retención por variante, con 10.000 payloads puntero y un GC forzado. El delta de heap incluye ruido; no es un presupuesto ni una comparación estadística.
- Enums, opcionales nativos y Result se comparan con structs tagged Go explícitos que distinguen presencia/fallo por tag, incluidos payloads nil presentes y Err(nil); no se asumen equivalentes a nil o tuplas Go.
- Recompilación: dependencias calientes, cambio del entero buildNonce impreso por main que invalida el paquete principal y el enlace, compile+link con -p=1 y -trimpath. No mide construir el toolchain ni una compilación con toda la caché vacía.
- Build sin cambios usa la caché y el ejecutable existente. Wall time incluye el comando público y, en Gon, su launcher. CPU es user+system informado por /usr/bin/time.
- RSS es el máximo informado por /usr/bin/time, no la suma simultánea de memoria de todos los subprocesos. Ejecutables sin stripping; incluyen runtime y metadatos de depuración.
- Intervalos 95%: bootstrap descriptivo de medianas de ratios por ronda (5000 remuestreos). No corrigen múltiples comparaciones ni garantizan generalización.
- Resultados de una sola máquina/sesión, sin fijar núcleo ni controlar frecuencia, carga externa o estado térmico. No prueban rendimiento en otras arquitecturas o aplicaciones.

Correctness: 16 tests per variant; identical stdout:

```text
build=0 items_per_batch=64 checksum=47764 failures=272 effects={Reads:384 Validations:240 Defaults:212 Arguments:59 Locations:128 TrueBranches:80 FalseBranches:128 Constructions:64 Copies:64 MatchGuards:21 MatchBodies:128 OptionReads:128 OptionTransforms:64 OptionDefaults:96 ResultReads:192 ResultTransforms:64 ResultHandlers:32 ResultDefaults:32 NilFailures:16 Callees:64 NamedArguments:192 ArgumentTrace:6}
```

Raw evidence: [runtime samples](runtime-raw.json), [build samples](build-raw.json), [representation](representation.json), [summary](summary.json), [commands](commands.json), [HTML](report.html). Source copies and logs are in the same run directory.
