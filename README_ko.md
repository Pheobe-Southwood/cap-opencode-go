<div align="center">

<img src="assets/logo.svg" width="88" alt="OpenCode Go">

# OpenCode Go Provider 플러그인

**OpenCode Go의 37개 전체 모델을 CLIProxyAPI에 완벽하게 연동하는 네이티브 플러그인**

[ 简体中文 ](README.md) | [ English ](README_en.md) | [ 日本語 ](README_ja.md) | [ Français ](README_fr.md) | [ 한국어 ](README_ko.md)

<p>
  <img src="https://img.shields.io/badge/CPA-v8.0.27+-blue?style=flat-square" alt="CPA Version">
  <img src="https://img.shields.io/badge/OpenCode_Go-37_Models-success?style=flat-square" alt="Models">
  <img src="https://img.shields.io/badge/Protocol-4_Families-orange?style=flat-square" alt="Protocols">
  <img src="https://img.shields.io/github/v/release/Pheobe-Southwood/cpa-opencodego?style=flat-square" alt="Release">
  <img src="https://img.shields.io/npm/v/cpa-opencodego?style=flat-square" alt="npm">
  <img src="https://img.shields.io/badge/License-MIT-lightgrey?style=flat-square" alt="License">
</p>

</div>

---

> **v2.0.3**: CPA 네이티브 키 스케줄링(`TargetKind: provider` 및 키당 4개 인증 파일 투영)을 복원하여 할당량 소진 후 키가 전환되지 않던 문제를 수정했습니다. 호스트 `quota_provider`는 복원하지 않습니다. 업그레이드 전에 [롤백 안내](docs/rollback.md)를 확인하세요.


## 핵심 가치와 주요 특징

OpenCode Go는 최신 고성능 AI 모델 37종(DeepSeek V4.1, Claude Haiku 5.5, Gemini 4 Argon, GPT 6 Luna, Grok 4.7, Qwen 3.8, Kimi K3 등)을 한곳에 모아 제공합니다. 그러나 업스트림 모델들이 4개의 서로 다른 프로토콜 엔드포인트로 나뉘어 있고, 모델마다 네트워크 프록시 요구 조건이 상이합니다.

본 플러그인은 CLIProxyAPI (CPA) 환경에서 다음과 같은 완벽한 통합 솔루션을 제공합니다:
- **4대 프로토콜 패밀리 완벽 지원**: `chat/completions`, `responses`, `messages`, `generateContent` 4개 엔드포인트를 완벽히 통합합니다. 클라이언트는 어떤 프로토콜 형식으로든 모든 모델을 자유롭게 호출할 수 있습니다.
- **모델별 개별 프록시 제어**: 모델 단위로 「프록시 경유」, 「직접 연결」, 「글로벌 설정 상속」을 독립적으로 지정할 수 있어 지역 제한과 네트워크 최적화에 완벽히 대응합니다.
- **네이티브 임베디드 관리 웹 대시보드**: CPA 관리 센터 내 전용 대시보드를 제공하여 멀티 키 우선순위 스케줄링, 실시간 쿼터 잔여량, 실측 TPS 및 첫 글자 지연(TTFT)을 한눈에 파악할 수 있습니다.
- **무결성 및 잔여물 없는 깔끔한 구조**: 순수 동적 라이브러리(`.so`) 방식으로 동작하여 CPA 소스 코드나 내부 데이터베이스를 일절 수정하지 않으며, 삭제 시 잔여 파일이 남지 않습니다.

---

## 주요 기능 안내

### 1. 모델별 개별 프록시 선택 및 별칭(Alias) 설정

> **핵심 하이라이트**: 모델마다 독립된 네트워크 아웃바운드 경로를 지정할 수 있으며, 클라이언트를 위한 커스텀 별칭을 적용할 수 있습니다.

![모델별 개별 프록시 선택 및 별칭 설정](docs/screenshots/desktop-models.png)

- **모델별 맞춤 프록시 모드**:
  - `프록시 경유 (proxy)`: 특정 중계 프록시가 필요한 모델(GPT 6 Luna, Grok 4.7 등 Responses 계열)에 명확히 프록시를 적용합니다.
  - `직접 연결 (direct)`: 프록시를 건너뛰고 업스트림에 직접 연결합니다(Gemini 4 Argon Contributor 등 지역 차단 회피를 위해 직접 연결이 필수적인 모델에 적용).
  - `상속 (inherit)`: 플러그인 전용 프록시 또는 CPA 글로벌 프록시 설정을 그대로 따릅니다.
- **모델 별칭(Alias) 지정**: 외부 노출용 이름을 설정할 수 있습니다(예: `deepseek-v4.1-flash` 모델을 `DeepSeek-V4.1-Flash`로 재명명). 다운스트림 모델 목록과 라우팅에 별칭이 우선 적용됩니다.
- **프로토콜 자동 추론 및 실측 검증**: 상위에 신규 추가된 모델의 엔드포인트를 모델 ID 접두사로 자동 추론하며, 「프로토콜 감지」 버튼으로 최소 패킷을 전송해 실제 통신 가능 여부를 즉시 검증할 수 있습니다.

---

### 2. 멀티 키 풀 및 실시간 다중 윈도우 쿼터 모니터링

> **핵심 하이라이트**: 상위 업스트림의 5시간, 주간, 월간 쿼터 잔여량을 직관적인 프로그레스 바로 실시간 확인하고 멀티 키를 자동 관리합니다.

![멀티 키 풀 및 실시간 쿼터 모니터링](docs/screenshots/desktop-accounts.png)

- **다중 윈도우 쿼터 시각화**: OpenCode Go의 공식 `/usage` API를 직접 조회하여 5시간 롤링, 주간 한도, 월간 한도의 잔여 퍼센티지와 갱신 카운트다운을 명확하게 표시합니다.
- **우선순위 기반 키 스케줄링(CPA 네이티브)**: 여러 API 키를 등록하면 플러그인이 각 키를 4개의 CPA 인증 파일(chat/responses/messages/gemini)로 투영하고, CPA의 `AuthManager`가 `priority`와 호스트 `routing` 설정에 따라 선택·순환·429 쿨다운·페일오버를 수행합니다. 플러그인 자체는 키 순환을 구현하지 않습니다. CPA의 '인증 파일' 페이지에는 키당 4행이 표시됩니다.
- **화면 내 즉시 이름 변경**: 대시보드에서 키 이름을 클릭하여 바로 수정할 수 있으며, 서비스 중단 없이 설정이 즉시 핫 리로드됩니다.

---

### 3. 전방위 사용량 분석 및 실측 성능 통계

> **핵심 하이라이트**: 토큰 소모량, 정가 환산 비용, 엔드투엔드 전체 TPS, 순수 디코딩 TPS 및 첫 글자 지연(TTFT)의 완벽한 가시성을 제공합니다.

![전방위 사용량 및 성능 통계](docs/screenshots/desktop-stats.png)

- **실측 성능 지표**:
  - **전체 TPS**: 성공한 요청의 유효 출력 토큰 ÷ 호스트 총 소요 시간(네트워크 지연, 대기 시간 포함).
  - **생성 TPS**: 첫 번째 콘텐츠 프레임부터 마지막 콘텐츠 프레임까지의 시간으로 측정한 순수 모델 디코딩 속도.
  - **평균 TTFT**: 요청 전송 시점부터 첫 유효 토큰 수신까지의 소요 시간.
- **비용 및 한도 소진율**: 공식 요금표 스냅샷을 내장하여 Go / Go Plus 플랜 기준 모델별 월간 한도 소진율과 각 키별 잔여 금액을 정밀하게 표시합니다.
- **업데이트 알림**: 30일 경과 시 또는 상위 모델 목록 변경 시 대시보드 상단에 부드러운 알림을 띄워 데이터 정합성을 유지합니다.
- **요청 로그**: 새 탭에서 각 상류 시도의 시각, 입력/출력/합계 토큰, TTFT, 생성 TPS, 종단간 TPS, 상태와 오류를 한 줄씩 보여 줍니다. 실패 사유와 상류 `x-opencode-log-id` 도 확인할 수 있습니다.

---

### 4. 4대 프로토콜 패밀리 통합 아키텍처

> **핵심 하이라이트**: 단일 설정으로 4개의 프로토콜 인터페이스를 동시에 구동하여 클라이언트의 프로토콜 불일치 문제를 완벽히 해결합니다.

![4대 프로토콜 패밀리 통합 아키텍처](docs/screenshots/desktop-status.png)

- **4개 프로토콜 인터페이스의 일체화**: 메인 플러그인이 대시보드와 설정을 총괄하고, 3개의 종속 모듈(Responses, Messages, Gemini)이 런타임 스냅샷을 자동 공유하여 유기적으로 동작합니다.
- **자유로운 프로토콜 교차 호출**: OpenAI 형식 클라이언트에서 Claude 모델을 호출하거나, Anthropic 형식 클라이언트에서 Gemini 4나 DeepSeek을 호출하는 과정이 완벽히 자동 변환됩니다.
- **간결한 설정 구조**: 메인 플러그인에만 설정을 추가하면 종속 모듈은 무설정으로 동작하여 관리가 매우 편리합니다.

---

## 빠른 시작

### 1단계: 플러그인 라이브러리 배치

**방법 A(권장): npm으로 설치**

```bash
npx cpa-opencodego install --dir /path/to/CLIProxyAPI
```

**방법 B: 수동 복사**

[GitHub Releases](https://github.com/Pheobe-Southwood/cpa-opencodego/releases)에서 `cpa-opencodego-bundle_2.0.3_<goos>_<goarch>.zip`을 내려받아 4개 라이브러리를 CPA 플러그인 디렉터리(일반적으로 `plugins/linux/amd64/`)에 복사합니다:
- `cpa-opencodego-v2.0.3.so`
- `cpa-opencodego-responses-v2.0.3.so`
- `cpa-opencodego-messages-v2.0.3.so`
- `cpa-opencodego-gemini-v2.0.3.so`

### 2단계: 호스트 설정 추가

CPA의 `config.yaml` 파일 내 `plugins:` 섹션에 설정을 추가합니다:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      developer-role: "retry-only"
      reasoning-replay: "retry-only"
      request-log: true
      keys:
        - name: "primary-key"
          key: "oc_sk_your_api_key_here"
          priority: 10
          enabled: true
    cpa-opencodego-responses: { enabled: true }
    cpa-opencodego-messages:  { enabled: true }
    cpa-opencodego-gemini:    { enabled: true }
```

### 3단계: 관리 콘솔에서 바로 사용

1. CLIProxyAPI 서비스를 시작하거나 재시작합니다.
2. CPA 관리 패널을 열고 왼쪽 사이드바의 「플러그인」에서 **OpenCode Go** 메뉴를 클릭합니다.
3. 모델 활성화, 개별 프록시 경로 지정, 실시간 쿼터 및 성능 모니터링을 즉시 시작하세요!

---

## 심층 기술 문서 및 아키텍처

4개 `.so` 라이브러리의 설계 원리, C ABI 인터페이스, SSE 스트리밍 첫 프레임 캡처 메커니즘, 전체 설정 옵션 상세 및 빌드 가이드는 아래 문서를 참고하세요:

- 📖 [기술 상세 및 전체 설정 레퍼런스 (docs/TECHNICAL.md)](docs/TECHNICAL.md)
- 📐 [핵심 설계 사양 (docs/design.md)](docs/design.md)
- 🛡️ [무잔여 롤백 및 제거 가이드 (docs/rollback.md)](docs/rollback.md)
- 🧪 [인수 테스트 및 검증 리포트 (docs/acceptance.md)](docs/acceptance.md)

---

## 라이선스

이 프로젝트는 [MIT 라이선스](LICENSE)에 따라 배포됩니다.
