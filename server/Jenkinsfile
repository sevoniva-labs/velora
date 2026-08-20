pipeline {
  agent { label 'domestic-offline' }
  options {
    disableConcurrentBuilds(abortPrevious: true)
    timestamps()
  }
  environment {
    GOPROXY = 'https://goproxy.cn'
    GOSUMDB = 'sum.golang.org https://goproxy.cn/sumdb/sum.golang.org'
    NPM_REGISTRY = 'https://registry.npmmirror.com'
    npm_config_registry = 'https://registry.npmmirror.com'
  }
  stages {
    stage('Policy') {
      steps { sh 'make ci-policy' }
    }
    stage('Backend') {
      steps { sh 'make ci-go' }
    }
    stage('Frontend') {
      steps { sh 'make ci-web' }
    }
    stage('Deploy') {
      steps { sh 'make ci-deploy' }
    }
    stage('Security') {
      steps {
        sh 'make security-tools'
        sh 'make supply-chain-evidence'
      }
      post {
        always { archiveArtifacts artifacts: '.evidence/**', allowEmptyArchive: false, fingerprint: true }
      }
    }
  }
}
