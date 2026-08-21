pipeline {
  agent { label 'domestic-offline' }
  options {
    disableConcurrentBuilds(abortPrevious: true)
    timestamps()
    skipDefaultCheckout(false)
  }
  environment {
    GOPROXY = 'https://goproxy.cn'
    GOSUMDB = 'sum.golang.org https://goproxy.cn/sumdb/sum.golang.org'
    NPM_REGISTRY = 'https://registry.npmmirror.com'
    npm_config_registry = 'https://registry.npmmirror.com'
  }
  stages {
    stage('Repository gates') {
      steps {
        sh 'make verify'
        sh 'make check-prod-config'
      }
    }
    stage('Release image evidence') {
      when {
        anyOf {
          branch 'main'
          buildingTag()
        }
      }
      steps {
        // Release jobs must inject an internal digest and approved offline
        // Trivy/Cosign material; missing inputs fail closed.
        dir('server') {
          sh 'make release-evidence'
        }
      }
    }
  }
  post {
    always {
      archiveArtifacts artifacts: 'server/.evidence/**', allowEmptyArchive: false, fingerprint: true
    }
  }
}
