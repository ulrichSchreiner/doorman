import LockPersonIcon from '@mui/icons-material/LockPerson';
import { Alert, Box, Button, Sheet, Typography } from '@mui/joy';
import * as React from 'react';
import { Route, Routes, useNavigate } from "react-router-dom";
import { Captcha } from './Captcha';
import { OTPEnter } from './OTPEnter';
import { RegisterUser } from './RegisterUser';
import { RemoteApi } from './RemoteApi';
import { Signup } from './Signup';
import { TokenEnter } from './TokenEnter';
import { User } from './User';
import { WaitForPermission } from './WaitForPermission';


const remoteAPI = new RemoteApi(location.origin);

export const App = (props) => {
    const navigate = useNavigate();
    const [uid, setUid] = React.useState("");
    const [solution, setSolution] = React.useState("");
    const [imgdata, setImgData] = React.useState(null);
    const [opmode, setOPMode] = React.useState("");
    const [token, setToken] = React.useState("");
    const [tokenCreated, setTokenCreated] = React.useState(new Date());
    const [waitKey, setWaitKey] = React.useState("");
    const [privacyURL, setPrivacyURL] = React.useState("");
    const [imprintURL, setImprintURL] = React.useState("");
    const [waitSecs, setWaitSecs] = React.useState(60);
    const [serverMessage, setServerMessage] = React.useState("");
    const [showError, setShowError] = React.useState(false);
    const [passthrough, setPassthrough] = React.useState(null);
    const [captchaMode, setCaptchaMode] = React.useState("");

    React.useEffect(() => {
        remoteAPI.uisettings().then(s => {
            setImprintURL(s.imprint);
            setPrivacyURL(s.privacy_policy);
            setOPMode(s.operation_mode);
            setWaitSecs(s.duration_secs);
            setCaptchaMode(s.captcha_mode);
        })
    }, []);

    const reloadWindow = () => {
        window.history.replaceState(null, null, window.location.pathname + window.location.search)
        window.location.reload();
    }

    const handleCloseError = () => {
        setShowError(false);
    };

    const handleRemoteError = (cb) => async () => {
        try {
            await cb();
        }
        catch (e) {
            if (e?.data?.captcha) {
                setImgData(e.data.captcha)
            } else {
                setImgData(null);
            }
            if (e.message) {
                setServerMessage(e.message);
                setShowError(true);
            } else {
                // this can happen, when we do a request which passes through to our
                // now whitelisted backend
                window.location.reload();
            }
        }
    };

    const sendUserSolution = async (uid, solution) => {
        setSolution(""); // clear the solution in the UI
        setShowError(false);
        setToken("");
        let u = await remoteAPI.sendUser(uid, solution);
        if (u.reload) {
            location.reload();
            return
        }
        switch (opmode) {
            case "token":
                setTokenCreated(new Date(parseInt(u.data.created, 10) * 1000));
                navigate("/enterToken");
                return
            case "otp":
                navigate(u.register ? "/registerUser" : "/enterOTP");
                return
            case "link":
                setWaitKey(u.data?.key);
                navigate("/waitForPermission");
                return
        }
    }

    const userEntered = handleRemoteError(async () => {
        if (captchaMode != "") {
            let d = await remoteAPI.createCaptcha();
            setImgData(d.data.captcha);
            navigate("/captcha");
        } else {
            await sendUserSolution(uid, solution);
        }
    });

    const solutionEntered = handleRemoteError(async () => {
        await sendUserSolution(uid, solution);
    });

    const checkToken = handleRemoteError(async () => {
        await remoteAPI.checkToken(token);
        setPassthrough(<div></div>);
        reloadWindow()
    });
    const checkOTP = handleRemoteError(async () => {
        await remoteAPI.checkOTP(token);
        setPassthrough(<div></div>);
        reloadWindow()
    });

    const userChanged = (u) => setUid(u);
    const solutionChanged = (s) => setSolution(s);

    const doRegister = handleRemoteError(async () => {
        await remoteAPI.register(uid);
    });

    const routes = [
        {
            path: "/registerUser",
            exact: true,
            component: <RegisterUser
                userid={uid}
                onNoUser={() => navigate("/", { replace: true })}
            />,
            title: "Register",
            nextLabel: "Register",
            valid: () => true,
            submit: doRegister
        },
        {
            path: "/signup/:uid/:regtoken",
            exact: true,
            component: <Signup
                placeholder="Token"
                onValidateOk={() => navigate("/", { replace: true })}
            />,
            title: "Signup",
            nextLabel: "",
            valid: () => true,
            submit: () => { }
        },
        {
            path: "/enterToken",
            exact: true,
            component: <TokenEnter
                placeholder="Token"
                value={token}
                userid={uid}
                waitSecs={waitSecs}
                tokenCreated={tokenCreated}
                onNoUser={() => navigate("/", { replace: true })}
                onTimeout={() => navigate("/", { replace: true })}
                onTokenChange={(t) => setToken(t)}
                onTokenSubmit={checkToken}
            />,
            title: "Token",
            nextLabel: "Check",
            valid: () => token != "",
            submit: checkToken,
        },
        {
            path: "/enterOTP",
            exact: true,
            component: <OTPEnter
                placeholder="OTP"
                value={token}
                userid={uid}
                onNoUser={() => navigate("/", { replace: true })}
                onTokenChange={(t) => setToken(t)}
                onTokenSubmit={checkOTP}
            />,
            title: "One Time Password",
            nextLabel: "Check",
            valid: () => token != "",
            submit: checkToken,
        },
        {
            path: "/waitForPermission",
            component: <WaitForPermission
                userid={uid}
                waitkey={waitKey}
                onWaitReady={() => reloadWindow()}
                onNoUser={() => navigate("/", { replace: true })} />,
            title: "Wait",
            valid: () => uid != "",
            nextLabel: "",
            submit: () => { },
        },
        {
            path: "/captcha",
            exact: true,
            component: <Captcha
                placeholder="Solution"
                mode={captchaMode}
                value={solution}
                imgdata={imgdata}
                onSolutionChange={solutionChanged}
                onSolution={solutionEntered}
            />,
            title: "I'm not a robot",
            nextLabel: "Next",
            valid: () => (solution != ""),
            submit: solutionEntered,
        },
        {
            path: "/",
            exact: true,
            component: <User
                placeholder="User ID"
                value={uid}
                onUserChange={userChanged}
                onUserSubmit={userEntered}
            />,
            title: "Enter User ID",
            nextLabel: "Next",
            valid: () => uid != "",
            submit: userEntered,
        },
    ];

    if (passthrough != null) return passthrough;

    return (
        <Box
            sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center"
            }}>
            {showError && <Box sx={{ position: "fixed", display: 'flex', flexDirection: 'column', gap: 2, width: '100%', maxWidth: "350px" }}>

                <Alert
                    variant="soft" color="danger"
                    endDecorator={
                        <Button
                            onClick={handleCloseError}
                            size="sm"
                            variant="outlined"
                            color="danger"
                            sx={{
                                textTransform: 'uppercase',
                                fontSize: 'xs',
                                fontWeight: 'xl',
                            }}
                        >
                            Close
                        </Button>
                    }>
                    {serverMessage}
                </Alert>
            </Box>}
            <Box
                sx={{
                    borderRadius: "50%",
                    border: "1px solid grey",
                    backgroundColor: "white",
                    textAlign: "center",
                    marginBottom: "-20px",
                    marginTop: "70px",
                    padding: "5px",
                    zIndex: 11
                }}><LockPersonIcon /></Box>
            <Box sx={{
                flexGrow: 1,
                maxWidth: 300,
                minWidth: 300,
                borderRadius: "10px",
                padding: "5px",
                zIndex: 10
            }}>
                <Sheet variant='soft' sx={{
                    display: 'flex',
                    alignItems: 'center',
                    height: "55px",
                    color: "white",
                    fontSize: "90%",
                    borderRadius: "8px",
                    justifyContent: "center",
                    marginBottom: "4px"
                }}>
                    <Typography >
                        <Routes>
                            {routes.map((route, i) => (
                                <Route key={i} path={route.path} element={<React.Fragment>{route.title}</React.Fragment>}></Route>
                            ))}
                        </Routes>
                    </Typography>
                </Sheet>
                <Routes>
                    {routes.map((route, i) => (
                        <Route key={i} path={route.path} element={route.component} />
                    ))}
                </Routes>
                <Box sx={{
                    display: 'flex',
                    justifyContent: "space-around",
                    textAlign: "center",
                    marginTop: "10px"
                }}>
                    <Routes>
                        {routes.filter(r => r.nextLabel != "").map((route, i) => (
                            <Route key={i} path={route.path} element={
                                <Button
                                    onClick={route.submit}
                                    disabled={!route.valid()}>
                                    {route.nextLabel}
                                </Button>}>
                            </Route>
                        ))}
                    </Routes>
                </Box>
                <Box sx={{
                    display: 'flex',
                    alignItems: 'center',
                    fontSize: "80%",
                    justifyContent: "space-around",
                    marginTop: "20px",
                    fontFamily: 'Roboto'
                }}>
                    {privacyURL != "" && <div><a target="_blank" href={privacyURL}>Privacy policy</a></div>}
                    {imprintURL != "" && <div><a target="_blank" href={imprintURL}>Imprint</a></div>}
                </Box>

            </Box>
        </Box>
    )

}
