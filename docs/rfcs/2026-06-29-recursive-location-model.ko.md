# RFC: 재귀적 Location 모델

> 임시 한글 번역본. 정본은 [영문](./2026-06-29-recursive-location-model.md).

- **상태:** Draft
- **생성:** 2026-06-29
- **대상:** cht-go-lint
- **영향:** location 전략, `dependency/*` 룰, config 스키마

## 요약

고정 3축 location 모델(`Component` / `SubComponent` / `Layer`)을 **재귀 노드
트리**로 교체한다. 파일의 아키텍처상 위치는 경로 prefix 매칭으로 결정되는
*노드 체인*이 되며, 디렉토리별로 cascade되는 `.cht-go-lint.yaml`에 선언한다.

노드는 **두 필드**를 가진다 — `may_import`(이 노드가 import 가능한 노드들)와
`shared`(부모 서브트리의 노드가 나를 import 가능한지). 기본은 **deny**: 형제
노드는 엣지를 선언하지 않으면 격리된다. 심볼/패키지 가시성은 Go에 맡긴다
(`internal/`, 대문자/소문자). cht는 Go가 열어둔 *import 그래프*만 통제한다. 네
개의 구조적 dependency 룰(`module-isolation`, `layer-direction`,
`cross-boundary`, `subdomain-isolation`)은 트리 위 단일 검사로 통합된다.

변경은 additive이며 새 메이저 버전으로 배포된다. 핀 고정된 기존 사용자는
영향받지 않는다. `flat-pkg`/`nested-domain`은 프리셋으로 유지된다.

## 동기

이 한계들은 cht-go-lint을 `go-lib`에 적용하며 드러났다 — 첫 **라이브러리**
사용자이자 첫 **이질적** 내부 구조 사용자. (기존 사용자는 전부 서비스: 3개가
`nested-domain` 위 `channeltalk/msa-v2` 프리셋, 1개가 균일 서비스 레이어의
`flat-pkg`, 1개가 golangci만 쓰는 미니멀 config.)

1. **재귀 안 됨.** `Location`이 고정 3슬롯이라 깊이가 3에서 막힌다. 더 깊게
   중첩되는 기능(예: `kafka` → `consumer`/`producer` → 각자 내부)을 표현 못한다.
   `ch-app-store`조차 `domain / subdomain / layer`에서 멈추고 `subdomain`을 두
   번 중첩하지 않는다.

2. **SubComponent가 마커에 묶임.** 경로에 literal `subdomain/`이 있을 때만
   할당된다. 패키지 경로에 마커 디렉토리를 억지로 박는 건 라이브러리에
   부자연스럽다.

3. **레이어가 전역.** 레이어 이름 하나가 모든 컴포넌트에 단일 규칙을 가진다.
   `go-lib` 기능은 이질적(`kafka` ≠ `sqlrepo` ≠ `errors`)이라 단일 전역 어휘가
   안 맞는다 — 같은 이름(`pool`)이 기능마다 다른 의미.

4. **세 룰, 한 개념.** `module-isolation`, `layer-direction`, `cross-boundary`는
   다 *"파일 X가 패키지 Y를 import해도 되나?"* 를 라벨로 답한다. 하나로 합칠 수
   있다.

## 변경 범위

| 분류 | 항목 |
|---|---|
| **신규** | 노드트리 location 전략; 노드 체인 `Location`; 노드 스키마(`children`/`may_import`/`shared`); 통합 `dependency/import` 룰. |
| **통합** | `module-isolation` + `layer-direction` + `cross-boundary` + `subdomain-isolation` → 단일 `dependency/import` (4 → 1). |
| **삭제** | 노드별 `public` 표면, 별도 `foundations` 목록, `isolate` 플래그 — 가시성은 Go `internal/`, broadcast는 `shared`로 대체. |
| **재매핑** | `naming/*`, `structure/*`, `iface/*`, `ddd/*`가 `Component`/`Layer` 대신 노드 체인을 읽음. 새 룰 아님. |
| **유지** | 엔진(walk/parse/report/golangci), fix, 캐시; tier 개념(단순화). |

additive로 배포되는 모델 오버홀이지 단일 새 룰이 아니다. dependency 룰은
줄어든다(4 → 1).

## 배경 — 현재 모델

- `Location{ Component, SubComponent, Layer }` — `LocationStrategy`가 할당하는
  단일값 3슬롯.
- 각 dependency 룰이 파일·import에 `Location`을 할당하고 라벨 쌍을 비교. layers/
  components 선언 + 룰 활성화되면 **deny-by-default**(선언된 `may_import` 방향만
  허용; cross-component 내부 deny).
- **tier 게이트**가 전제 config 없는 룰을 skip.
- 파싱은 AST만, 파일당 캐시.

## 용어

**트리**

- **node(노드)** — 유닛으로 참여하는 디렉토리: 부모(*walling* 부모)가 벽 쳐주거나,
  자기가 *walling 노드*이거나.
- **root** — 최상위 노드; 항상 노드.
- **parent/child/sibling** — 트리 중첩; sibling은 같은 parent를 공유.
- **ancestor/descendant/subtree** — 체인 위/아래; subtree는 한 노드 + 모든 자손.
- **walling 노드** — config를 가진 디렉토리; 직속 하위를 자식 노드로 벽 친다. 로컬 —
  자기가 walled인지와 무관.
- **leaf(잎)** — 자식 없는 노드; 그 하위 디렉토리는 자기 코드지 노드 아님. (잎도
  노드다.)
- **chain(체인)** — 파일의 `Location`: root부터 파일 소유 노드까지, 예 `[root,
  kafka, consumer]`.

**그래프**

- **import** — 코드의 실제 Go `import`.
- **edge(엣지)** — 두 노드 사이 *선언된 허용* import 관계. 기본은 엣지 없음(deny);
  린트가 import마다 엣지에 대조.
- **`may_import`** — importer쪽 선언 유향 엣지 (`A may_import B` = 엣지 `A → B`).
- **`shared`** — broadcast 엣지: `B shared`면 `B`의 형제들(+그 서브트리) → `B` 엣지
  열림.
- **wall(벽)** — 형제 노드 간 기본 deny; 엣지는 벽의 구멍.

**정책**

- **deny-default** — 형제 노드는 엣지 없으면 막힘.
- **divergence(갈라짐)** — 두 체인이 갈리는 레벨; import 체크가 일어나는 형제 레벨.
- **visibility** — Go `internal/` + 대문자/소문자. cht 엣지와 별개; 표면은 Go에 위임.

## 제안 설계

### 노드 트리

아키텍처는 **노드** 트리다. 노드는 디렉토리에 대응하며 두 필드를 가진다:

| 필드 | 방향 | 의미 |
|---|---|---|
| `may_import` | 당기기(importer) | 이 노드가 import 가능한 노드들. 명시적·유향 엣지. |
| `shared` | 밀기(importee) | `true`면, **부모 서브트리** 안의 노드가 나를 import 가능. 공통 의존을 형제마다 적는 대신 한 번에. |

**무엇이 디렉토리를 노드로 만드나.** 자주 뭉뚱그려지는 두 가지를 분리한다:

- 디렉토리가 **자기 자식을 벽 친다** — *walling 노드*가 된다 — 는 건 config(
  `children` 섹션, inline 또는 co-located)를 가질 때다. 이건 **로컬**: 직속 하위가
  자식 노드가 되어 서로 기본 격리된다. `kafka/consumer/pool`에 config를 추가하면
  `pool`이 자기 서브트리의 walling 노드가 되지, `consumer`를 *승격시키지 않는다*.
  repo 루트는 항상 walling 노드.
- 디렉토리가 **형제와 격리(walled)**되는 건 그 *부모*가 walling 노드일 때뿐이다.
  그래서 `consumer`가 형제와 격리되는지는 `consumer`가 아니라 `kafka`의 몫 — 자기
  레벨의 노드성은 **부모의** config가 준다.

config 없고 부모가 leaf인 디렉토리는 그냥 코드다. config에서 이름 적는 건 정책
(`may_import`/`shared`)을 붙일 뿐; 이름 안 적은 직속 하위도 노드다(deny-default, 엣지
없음). 노드 `path`는 그냥 그 디렉토리(예 `pkg/kafka/consumer`)이고 parent/child는 경로
prefix로 따라온다.

`shared` 범위는 **위치**가 정한다 — "전역 vs 형제" 설정 없음:

- repo 루트 직속 `shared` 노드 → 어디서나 import 가능 (옛 `foundations`, 예
  `errors`);
- `kafka` 안 `shared` 노드 → `kafka` 안에서 import 가능 (옛 per-node `shared`,
  예 `kafka/core`).

**엣지는 공통 부모가 선언한다.** 한 형제 집합의 `may_import`/`shared`는 그들이
함께 놓인 *공통 부모* config에 산다 — 자식 자기 파일이 아니라. `consumer →
producer`는 `kafka` config(의 `consumer` 키 아래)에 선언되고, 분리된
`kafka/consumer/.cht-go-lint.yaml`은 `consumer`의 *자기* 자식들만 선언하지 형제를
참조하지 않는다. 이러면 각 레벨 배선이 한 곳에 보이고, 의존 매니저가 deps를 각
유닛에 흩뿌리는 것(Go import, Bazel, Maven)이 아니라 아키텍처 린터가 정책을
중앙화하는 방식(Nx 모듈경계, ArchUnit, depguard)을 따른다. 룰 의미는 그대로 —
`may_import`은 여전히 노드 `S`의 나가는 집합이고, *선언 위치*만 부모다.

### 기본 정책

기본은 **deny**: 형제 노드는 격리. 노드 `S`의 파일이 노드 `T`의 패키지를 import하는
경우를 보자.

- **수직은 항상 열림.** `S`, `T` 중 하나가 다른 쪽의 조상이면(같은 root→leaf 선상),
  허용 — 노드는 자기 서브트리를 보고, 자기를 감싸는 기능까지 올라가 쓸 수 있다.
  `kafka`가 자기 `consumer` 자식을 쓰거나 `consumer`가 `kafka`의 공유 타입을 쓰는
  건 항상 OK.
- **수평은 갈라지는 지점에서 체크.** 아니면 `S`와 `T`는 어느 레벨에서 갈린다: `Sc`,
  `Tc`를 그 *최저 공통 조상*의 자식인 형제 노드라 하자. 다음이면 허용:
  - `Tc ∈ Sc.may_import` — 공통 부모에 선언된 형제 엣지 `Sc → Tc`. 항목은 *형제만*
    이름 적는다; `Tc`의 *얼마나*가 닿는지는 더 깊은 `may_import` 경로가 아니라 Go
    `internal/`이 정한다. 또는
  - `Tc`가 `shared`이고 `Sc`가 그 부모 서브트리 안.

  아니면 위반.

`isolate` 플래그 불필요 — deny가 기본이고, `may_import`/`shared`가 엣지를 여는 법.
체크가 항상 **갈라지는 레벨의 형제**에 떨어지므로 같은 룰이 모든 깊이를 커버한다:
깊은 `kafka/consumer/x`가 `sqlrepo`를 import하면 `kafka`-vs-`sqlrepo` 체크로
환원되고(`consumer`가 아니라 `kafka.may_import`가 관장), `consumer ⊥ producer`는
`kafka`에서 환원된다. 부모가 자식을 import한다고 자식의 형제를 잇지 않는다:
`kafka`가 `consumer`·`producer` 둘 다 써도 `consumer`가 `producer`를 import하게 되지
않는다.

**가시성은 Go의 일이고 cht는 중복하지 않는다.** Go가 이미 `internal/`(서브트리
밖 도달 불가), 대문자/소문자, 비순환 import, 모듈 경계를 강제한다. cht는 Go가
열어둔 유향 import 그래프만 더한다. 노드는 privates를 `internal/`로 숨기고, cht
검사는 Go 가시성 *위에서* 돈다(대체가 아님).

따라서 **cross-feature 노출**에 `public` 필드가 불필요하다. `sqlrepo`가
`kafka/core`를 쓰려면 루트 config(둘의 공통 부모)가 `sqlrepo: { may_import:
[kafka/core] }`를 선언한다. Go `internal/`이 무엇을 선언하든 `kafka` privates를
도달 불가로 지킨다.

### Location 할당

파일 `Location`은 **경로 위의 노드 체인**, 가장 깊은 노드가 소유. 노드가 아닌
디렉토리(leaf 아래에 있음)는 위쪽 가장 가까운 노드에 속함. 마커 없음; 깊이 무제한.

```text
file:  pkg/kafka/consumer/pool/x.go
chain: [kafka, kafka/consumer]          # kafka, consumer가 노드
                                        # consumer가 leaf → pool은 consumer 코드
```

### Config 배치

어디서나 파일명 하나: **`.cht-go-lint.yaml`**, 디렉토리별 cascade(`.eslintrc` /
`.editorconfig`처럼).

- **루트** `.cht-go-lint.yaml` — 전역 필드(`module`, `rules`, golangci)와 루트
  노드 본문(단순한 경우 inline 노드 선언).
- **Co-located** 기능 디렉토리 안 `.cht-go-lint.yaml` — 그 디렉토리의 *자식*을 배선
  (경로는 위치로 암시). 형제로의 엣지는 여전히 공통 부모 config에, 여기 아님.
- **노드당 한 곳:** 노드는 *정확히 한 곳*에 선언된다 — 조상 config에 inline *또는*
  자기 co-located 파일, 둘 다 아님. 같은 노드를 두 번 선언하면 에러라, merge/우선순위
  고민이 없다. (루트 vs 노드는 내용으로 구분 — 루트가 `module:` 가짐.)
- **전역 룰:** 룰 on/off·severity(naming, `forbidden-dirs` 등)는 루트 `rules:`
  섹션에, repo 전역. 전역 *import 도달*은 루트의 `shared` 노드(예 `errors`)면 됨 —
  그 외 별도 "전역" 개념 불필요.
- **Top-level roots:** 코드는 보통 `pkg/` 같은 prefix 아래 있다. `roots: [pkg]`
  옵션(`flat-pkg`에서 이어옴)이 루트 노드의 top-level 기능 자식이 어디서 시작하는지
  알려, `pkg`가 한 노드가 아니라 `pkg/kafka`·`pkg/sqlrepo`가 루트의 자식이 된다.
- **Severity override:** 노드 config는 자기 서브트리에 룰 severity를
  설정(cascade)할 수 있다 — 예: 전역 기본이 `error`인데 레거시 노드만 `warn`. 현
  도구의 컴포넌트별 severity override와 동일.

**권장:** 작고 중앙 소유 repo(`go-lib`)는 단일 루트 파일; 큰 다팀
repo(`ch-app-store`)는 기능별 co-locate. 기능 분리는 스케일 탈출구지 기본 아님.

### 발견 & 조립

시작 단계가 트리를 순회하며 `.cht-go-lint.yaml`을 찾고, 루트 inline 노드와 병합,
경로로 트리 조립. 싸고(YAML만) run당 1회; 트리가 모든 룰의 location 전략이 됨.
순회는 analyzer 기존 제외(내장 skip: `vendor`, `testdata`, `.git`, `generated`,
`node_modules` + 설정된 `exclude_paths`)를 재사용해 vendored/generated config가
트리에 안 들어옴. Bazel이 `BUILD` 로드 후 동작하는 것 반영 — Rust `mod`/`pub`,
Java JPMS, Nx 경계에서도 보이는 패턴.

```text
1. 루트 config 로드
2. .cht-go-lint.yaml 발견 (+ 루트 inline 노드)        # 신규
3. 경로로 노드 트리 조립                              # 신규
4. 트리를 location 전략으로 analyzer 생성
5. 룰 루프: .go 순회, 트리로 Location 할당, 검사
6. golangci 통합
```

5단계 `.go` 순회는 같은 제외를 적용하고 추가로 `_test.go` 파일을 skip한다(현 엔진과
동일).

### 통합 dependency 룰

내부 import에 대해, 엔진은 source/target 노드 체인을 받아 기본 정책 검사(수직 →
열림; 아니면 갈라지는 레벨의 형제 엣지/`shared`)를 적용하고, 실패면 위반으로
보고한다. 이 단일 검사가 `module-isolation`(형제 격리), `layer-direction`(`may_import`
방향), `cross-boundary`(표면은 Go `internal/`), `subdomain-isolation`(어느 깊이든
형제 격리)을 흡수.

### 모듈 추출

파일명·노드 문법이 통일돼 노드의 `.cht-go-lint.yaml`은 이미 거의 루트 config.
기능을 자기 Go 모듈로 추출 = 재작성이 아니라 **승격**(`module:` 추가):

```yaml
# 이전 — go-lib 안 노드 (pkg/kafka/.cht-go-lint.yaml)
children: { core: { shared: true }, producer: {...}, consumer: { may_import: [producer] } }

# 이후 — 자기 모듈 루트 (kafka/.cht-go-lint.yaml)
module: github.com/channel-io/go-kafka    # 추가된 유일한 줄
children: { core: { shared: true }, producer: {...}, consumer: { may_import: [producer] } }
```

각 모듈은 *각자* 분석된다 — 자기 `.cht-go-lint.yaml`, 자기 트리, 자기 run(`go-lib`은
이미 메인 모듈 + `auth` 모듈). 모듈 간 import는 external이고 Go 모듈 시스템이
관장한다; **cht는 모듈 경계를 넘지 않는다.** (추출된 노드를 트리에 남겨 경계 너머
`may_import`을 강제하는 "span 모드"는 다중모듈 분석이 필요 — v1 범위 밖.)

## 예시

디렉토리:

```text
pkg/
├── errors/                 # root에서 shared → 어디서나 import 가능
├── kafka/
│   ├── .cht-go-lint.yaml    # 이 dir = kafka 노드
│   ├── kafka.go             # kafka 공개 API (Go: 대문자)
│   ├── core/                # kafka 안에서 shared
│   ├── producer/
│   ├── consumer/
│   └── internal/            # Go가 kafka 밖에서 숨김
└── sqlrepo/
```

Config:

```yaml
# pkg/kafka/.cht-go-lint.yaml  (경로 암시 = kafka)
children:
  core:     { shared: true }                  # consumer/producer가 import 가능
  producer: {}
  consumer: { may_import: [producer] }         # consumer → producer (한 방향)
```

- `kafka/consumer`는 `kafka/producer`와 `kafka/core`를 import 가능;
  `kafka/producer`는 `kafka/consumer` 불가.
- 둘 다 `pkg/errors` import 가능(root에서 shared).
- `kafka ⊥ sqlrepo` 기본; `sqlrepo`가 `kafka`를 쓰려면 **루트** config(둘의 공통
  부모)가 `sqlrepo: { may_import: [kafka] }` 선언 — `may_import`은 *형제* 이름이라
  `kafka` 통째 허용이고, 실제 얼마나 닿는지는 Go `internal/`이 정한다(여기선
  비-internal `core`만).

## 룰 재매핑 (41개)

- **`dependency/*`:** `module-isolation` + `layer-direction` + `cross-boundary`
  + `subdomain-isolation` → 단일 import 룰. `forbidden-imports`, `infra-in-core`,
  `handler-*`, `*-service-*`는 `may_import` 제약 or 경로/옵션 룰로.
- **`naming/*`, `structure/*`, `iface/*`, `ddd/*`:** 원칙 — `dependency/*`는 단일
  import 룰이 되고, `naming/*`은 golangci(revive)에 위임, 서비스형
  `structure/*`/`iface/*`/`ddd/*`는 서비스 프리셋에만(go-lib 같은 라이브러리는 off).
  노드 체인으로의 룰별 정확한 매핑(가장 깊은 노드 ≈ 오늘날 컴포넌트)은 기계적이고
  *구현 단계*에서 정한다 — 설계 질문 아님.
- **tier 게이트:** 컴포넌트·레이어가 노드로 통합되어 layer-aware/component-aware
  구분이 "노드 있나" 단일 게이트로 붕괴. 구현 시 재검토.

## 하위 호환

- `flat-pkg`/`nested-domain`은 경로 컨벤션에서 노드 트리를 생성하는 **프리셋**으로
  유지 → 기존 config 그대로.
- **버전 핀이 안전망.** 사용자는 핀 버전(`go install …@vX`) 설치. 노드 모델을 새
  메이저(`v1`)로 내면 기존 사용자 무영향; 준비되면 bump.
- **점진 채택은 오늘과 동일** — 아키 severity를 `warn`으로 시작해 정리하며
  `error`로 승격.

## 검토한 대안

- **open-default + `isolate` 플래그.** 검토: 형제를 Go처럼 open, 노드별 strict
  opt-in. **deny-default**(현 도구 방향) 택해 기각: `may_import`은 deny baseline
  위에서만 의미 있고, deny-default가 강제를 opt-in이 아니라 기본으로 켬. 점진
  채택은 `warn` severity로.
- **노드별 `public` 표면.** 삭제 — Go `internal/` + 대문자가 이미 표면을 정의;
  `public` 목록은 Go 중복. cross-feature 사용은 importer `may_import`로.
- **`foundations` 목록 / `visible_to: all` 범위.** 삭제 — root의 `shared` 노드가
  이미 전역; 위치가 범위.
- **두 primitive (`module` / `layer`).** 노드 모델 위 선택적 라벨/sugar; primitive
  아님.
- **경로 glob 엣지 그래프.** 기각 — 읽히는 의미적 아키텍처를 잃고 강화된
  `depguard`로 격하.
- **3축 모델 내 점진.** 기각 — 재귀·마커 미해결. (초기 component-scoped-layers PR은
  이 RFC 위해 닫음.)

## 보류

- **마이그레이션 도구.** 기존 레포는 `flat-pkg`/`nested-domain` 프리셋으로 그대로
  돌아가니 채택에 변환기는 불필요. 옛 config를 명시적 노드 트리로 다시 쓰는 도구를
  낼지는 열어둠 — 수동 마이그레이션 한 번 해보고 결정.

## 마이그레이션 계획

1. RFC 리뷰.
2. 노드트리 전략과 통합 import 룰을 새 config 뒤에 구현(additive; 현재 전략
   무수정).
3. `flat-pkg`/`nested-domain`을 새 모델 위 프리셋으로 배포.
4. `v1`(메이저) 릴리스. `go-lib`이 단일 루트 파일로 먼저 채택; 서비스는 준비되면
   핀 bump.
