
const app = new Vue({
    el: '#app',
    data: {
        repositoryList: [],
        teamList: [],
        search: "",
        loading: true,

        selectedTeams: [],
        selectedRepository: "",

        errors: [],
        status: ""
    },
    watch: {
        repositoryList: function (val) {

        },
    },
    delimiters: ['[[', ']]'],
    computed: {
        organization() {
            return this.selectedRepository !== "" ? this.selectedRepository.split("/")[0] : "";
        },
        filteredList() {
            return this.repositoryList.filter(repository => {
                return repository.name.toLowerCase().includes(this.search.toLowerCase()) ||
                    repository.full_name.includes(this.search.toLowerCase()) ||
                    ("https://github.com/" + `${repository.full_name}`).includes(this.search.toLowerCase())
            })
        },
        showTeams() {
            return this.search !== "" && this.filteredList.length === 1
        },
        readyToSubmit() {
            return this.showTeams && this.selectedTeams.length > 0
        },
    },
    methods: {
        getRepositories: function () {
            this.status = "Fetching repository list, hang on..."
            this.repositoryList = []
            this.errors = []

            axios.get('/proxy/repositories', {
                before: () => {
                    this.loading = true
                    this.status = "Fetching repositories from GitHub"
                }
            }).then(response => {
                this.repositoryList = response.data
            }).then(() => {
                this.loading = false
                if (!this.repositoryList) {
                    alert("Could not load any repositories, please sign back in");
                    window.location = "/auth/logout";
                }
                if (!this.repositoryList.length) {
                    this.errors.push("You don't have the sufficient access to any repo, please check your permissions")
                }
            }).catch((error) => {
                this.errors.push("An error occurred when fetching repositories, please logout and sign back in.")
                this.errors.push(error)
            })
        },
        getTeams: function (repository) {
            this.teamList = []
            this.errors = []

            this.loading = true
            this.status = "Fetching teams from GitHub for repo " + repository

            axios.get("/proxy/teams?repository=" + repository).then(response => {
                this.loading = false
                if (!response.data)
                    this.errors.push("Failed to access teams for " + repository +
                        ". This could be due to the fact that you are an admin/creator of the repository, but you don't have maintainer access to the team you've selected")
                if (!response.data.length) {
                    this.errors.push("You are not listed as an admin in any teams in the selected repository.\n " +
                        "Either choose a different repository, or have a team maintainer add you to the correct team.")
                }
                this.teamList = response.data
            }).catch((error) => {
                this.errors.push("Failed to access teams for " + repository + ". Error " + error)
            })

        },
        onChange(event) {
            const repo = event.target.value
            this.search = repo
            this.getTeams(repo)
        },
        checkForm: function (e) {
            this.errors = []

            if (this.selectedRepository && this.selectedTeams) {
                return true
            }

            if (!this.selectedRepository) {
                this.errors.push("Please select a repository")
            }

            if (!this.selectedTeams) {
                this.errors.push("Please selected a team")
            }

        },
        clear() {
            this.search = ''
            this.selectedRepository = ''
            this.selectedTeams = []

            this.errors = []
        }
    },
    mounted() {
        this.getRepositories()
    }
});
