@Library('homelab-jenkins-library@main') _

pipeline {
    agent {
        kubernetes {
            yaml homelab.podTemplate('default')
        }
    }

    stages {
        stage('Build') {
            steps {
                sh 'go build ./cmd/htd'
            }
        }

        stage('Test') {
            steps {
                sh 'go test ./...'
            }
        }
    }

    post {
        failure {
            script {
                homelab.notifyDiscord(status: 'FAILURE')
            }
        }
    }
}
