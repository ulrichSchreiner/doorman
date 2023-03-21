import { Box, FormControl, Input } from '@mui/joy';
import * as React from 'react';
import { useNavigate, useParams } from "react-router-dom";
import { RemoteApi } from './RemoteApi';

const remoteAPI = new RemoteApi(location.origin);

interface CaptchProps {
    placeholder: string
    imgdata: string
    onSolution: () => void
    onSolutionChange: (s: string) => void
    mode: string
    value: string
}

export const Captcha = ({ placeholder, imgdata, onSolution, onSolutionChange, mode, value }: CaptchProps) => {
    const { uid, regtoken } = useParams();
    const navigate = useNavigate();

    const checkEnter = (evt: React.KeyboardEvent) => {
        if (evt.key === "Enter") {
            onSolution()
            return false;
        }
        return true;
    }

    let description = "Enter the image text";
    if (mode == "math") {
        description = "Enter the image solution";
    }

    if (!imgdata) {
        navigate("/", { replace: true })
        return null;
    }

    return (
        <Box sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
        }}>
            {imgdata != null &&
                <Box sx={{
                    display: "flex",
                    justifyContent: "center",
                    flexDirection: "column",
                    alignItems: "center",
                }}>
                    <Box sx={{
                        marginTop: "20px",
                        marginBottom: "20px",
                    }}><img width="250" height="50" alt="Signup" src={"data:image/png;base64," + imgdata}></img></Box>
                    <Box sx={{
                        marginTop: "5px",
                        marginBottom: "5px",
                        fontFamily: 'Roboto',
                    }}>{description}</Box>
                    <FormControl>
                        <Input
                            placeholder={placeholder}
                            autoFocus
                            value={value}
                            onKeyDown={(evt) => checkEnter(evt)}
                            onChange={(evt) => onSolutionChange(evt.target.value)} />
                    </FormControl>

                </Box>
            }
        </Box>
    );
}