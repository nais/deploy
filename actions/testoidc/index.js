const core = require('@actions/core')

const decode = (token) => [
	JSON.parse(Buffer.from(token.split('.')[0], 'base64').toString()),
	JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString())
]

try {
	const aud = core.getInput('aud')
	core.getIDToken(aud).then(token => {
		const claims = decode(token)
		core.setOutput('result', claims)
	})
} catch (err) {
	core.setFailed(err.message)
}
