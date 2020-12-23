const handleResponse = (response => {
    return response.json().then(r => {
        if (!response.ok) throw (r)
        return r;
    })
})

export class RemoteApi {
    constructor(base) {
        this.base = base;
    }

    async sendUser(uid) {
        let fd = new FormData();
        fd.append("uid", uid);

        return fetch(this.base + "/sendUser", {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse)
    }

    async register(uid) {
        let fd = new FormData();
        fd.append("uid", uid);

        return fetch(this.base + "/register", {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async fetchTempRegister(uid, key) {
        let fd = new FormData();
        fd.append("uid", uid);
        fd.append("key", key)

        return fetch(this.base + "/fetchTempRegister", {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }
    async validateTempRegister(uid, key, token) {
        let fd = new FormData();
        fd.append("uid", uid);
        fd.append("key", key)
        fd.append("token", token)

        return fetch(this.base + "/validateTempRegister", {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async checkToken(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + "/checkToken", {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async checkOTP(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + "/checkOTP", {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async waitFor(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + "/waitFor", {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async uisettings() {
        return fetch(this.base + "/uisettings").then(handleResponse)
    }

}