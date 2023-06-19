const core = require('@actions/core')
const jose = require('jose')

try {
	const aud = core.getInput('aud')
	core.getIDToken(aud).then(token => {
		const claims = jose.decodeJwt(token)
		core.setOutput('result', claims)
	})
} catch (err) {
	core.setFailed(err.message)
}
