const core = require('@actions/core')

try {
	const aud = core.getInput('aud')
	console.log(`hello future audience ${aud}`)
	core.getIDToken(aud).then(token => {
		console.log(`got: ${token}`)
		core.setOutput("token", token)
	})
} catch (err) {
	core.setFailed(err.message)
}
