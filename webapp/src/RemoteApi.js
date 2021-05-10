const handleResponse = (response => {
    return response.json().then(r => {
        if (!response.ok) throw (r)
        return r;
    })
})

const dmrequest = "__dm_request__";

export class RemoteApi {
    constructor(base) {
        this.base = base;
    }

    async sendUser(uid, captcha) {
        let fd = new FormData();
        fd.append("uid", uid);
        fd.append("captcha", captcha);
        fd.append(dmrequest, 1);

        return fetch(this.base + `/sendUser?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse)
    }

    async register(uid) {
        let fd = new FormData();
        fd.append("uid", uid);

        return fetch(this.base + `/register?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async fetchTempRegister(uid, key) {
        let fd = new FormData();
        fd.append("uid", uid);
        fd.append("key", key);

        return fetch(this.base + `/fetchTempRegister?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async createCaptcha() {
        let fd = new FormData();

        return fetch(this.base + `/createCaptcha?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async validateTempRegister(uid, key, token) {
        let fd = new FormData();
        fd.append("uid", uid);
        fd.append("key", key);
        fd.append("token", token);

        return fetch(this.base + `/validateTempRegister?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd,
        }).then(handleResponse);
    }

    async checkToken(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + `/checkToken?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async checkOTP(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + `/checkOTP?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async waitFor(token) {
        let fd = new FormData();
        fd.append("token", token);

        return fetch(this.base + `/waitFor?${dmrequest}=1`, {
            method: 'POST',
            cache: 'no-cache',
            body: fd
        }).then(handleResponse);
    }

    async uisettings() {
        return fetch(this.base + `/uisettings?${dmrequest}=1`).then(handleResponse)
    }

}