import { Box, Button, FormControl, Input } from '@mui/joy';
import * as React from 'react';
import { Link, useParams } from "react-router-dom";
import { RemoteApi } from './RemoteApi';

const remoteAPI = new RemoteApi(location.origin);

interface SignupProps {
    placeholder: string
    onValidateOk: () => void
}
export const Signup = ({ placeholder, onValidateOk }: SignupProps) => {
    const { uid, regtoken } = useParams();
    const [imgdata, setImgData] = React.useState(null);
    const [message, setMessage] = React.useState("");
    const [token, setToken] = React.useState("");

    React.useEffect(() => {
        const fetchdata = async () => {
            try {
                let d = await remoteAPI.fetchTempRegister(decodeURIComponent(uid), decodeURIComponent(regtoken));
                setImgData(d.data.image);
                setMessage(null);
            } catch (err) {
                setMessage(err.message);
            }
        }
        fetchdata()
    }, []);

    const validate = async () => {
        try {
            await remoteAPI.validateTempRegister(decodeURIComponent(uid), decodeURIComponent(regtoken), token);
            onValidateOk();
        }
        catch (err) {
            setMessage(err.message);
        }
    }

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            validate();
            return false;
        }
        return true;
    }

    let msg = null;
    if (message) {
        msg = (<div>
            <Box sx={{
                marginTop: "5px",
                marginBottom: "5px",
                fontFamily: 'Roboto',
                fontWeight: "bold"
            }}>{message}</Box>
            <div>It seems your registration timed out, please <Link to="/">register again.</Link></div>
        </div>)
    }

    return (
        <Box sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
        }}>
            {msg}
            {imgdata != null &&
                <Box sx={{
                    display: "flex",
                    justifyContent: "center",
                    flexDirection: "column",
                    alignItems: "center",
                }}>
                    <img width="200" height="200" alt="Signup" src={"data:image/png;base64," + imgdata}></img>
                    <div>Please scan the QR code and enter the token shown by your device. Then click 'Validate'</div>
                    <FormControl>
                        <Input
                            placeholder={placeholder}
                            autoFocus
                            value={token}
                            onKeyDown={(evt) => checkEnter(evt)}
                            onChange={(evt) => setToken(evt.target.value)} />
                    </FormControl>
                    <Button onClick={validate}>
                        Validate
                    </Button>
                </Box>
            }
        </Box>
    );
}