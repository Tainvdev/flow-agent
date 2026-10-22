"use strict";
this.default_AiSandboxAngularFrontend = this.default_AiSandboxAngularFrontend || {};
(function(_) {
    var window = this;
    try {
        var CL, TVa, VVa, SVa, WVa, $Va, dWa, fWa, eWa, gWa, hWa;
        _.NVa = function(a) {
            var b = c => Math.min(a.Aja() * 2 ** c + Math.random() * a.Cja(), a.sna);
            return () => a.isEnabled() ? {
                retryDelay: b
            } : {}
        }
        ;
        _.sL = function(a) {
            return a.type === _.rL
        }
        ;
        _.uL = function(a) {
            return a.type === _.tL
        }
        ;
        _.vL = function(a) {
            a = a.replace(/<system[^>]*>[\s\S]*?<\/system>/gi, "");
            var b = a.search(/<system/i);
            b >= 0 && (a = a.substring(0, b));
            return a.replace(/[ \t]{2,}/g, " ")
        }
        ;
        _.wL = function(a) {
            return _.vL(a).trim()
        }
        ;
        _.OVa = function(a) {
            switch (_.Bw(a, _.xL)) {
            case 1:
                return (a = _.Fw(a, _.yL, 1, _.xL)) ? {
                    id: _.p(a, 1) || "",
                    handle: a.Yh() || "",
                    Fh: "media"
                } : null;
            case 2:
                return (a = _.Fw(a, _.zL, 2, _.xL)) ? {
                    id: _.p(a, 1) || "",
                    handle: a.Yh() || "",
                    Fh: "audio"
                } : null;
            case 3:
                return (a = _.Fw(a, _.AL, 3, _.xL)) ? {
                    id: a.Mh() || "",
                    handle: a.Yh() || "",
                    Fh: "entity"
                } : null;
            case 4:
                return (a = _.Fw(a, _.BL, 4, _.xL)) ? {
                    id: a.Sk() || "",
                    handle: a.Yh() || "",
                    Fh: "likeness"
                } : null;
            case 0:
                return null;
            default:
                return null
            }
        }
        ;
        CL = function(a) {
            return typeof a === "object" && a !== null
        }
        ;
        _.EL = function(a) {
            if (!a)
                return "";
            if (typeof a === "string")
                return _.wL(a);
            if (PVa(a)) {
                var b;
                let c, d, e, f = ((e = (d = (b = a.ka) == null ? void 0 : b.call(a)) != null ? d : (c = a.na) == null ? void 0 : c.call(a)) != null ? e : []).map(g => {
                    if (g.Tn())
                        return {
                            text: _.as(g, 1, _.DL),
                            Kt: !1
                        };
                    if (!g.Ct())
                        return {
                            text: "",
                            Kt: !1
                        };
                    var h, k, l, q = (l = (h = g.ka) == null ? void 0 : h.call(g)) != null ? l : (k = g.na) == null ? void 0 : k.call(g);
                    return {
                        text: q ? _.QVa(q) : "",
                        Kt: !0
                    }
                }
                ).filter(g => g.text !== "");
                a = f.reduce( (g, h, k) => {
                    if (k === 0)
                        return h.text;
                    k = f[k - 1];
                    if (!k)
                        return g + h.text;
                    k = k.Kt && (h.Kt || /^\w/.test(h.text)) || h.Kt && !k.text.endsWith(" ");
                    return g + (k ? " " : "") + h.text
                }
                , "");
                return _.wL(a)
            }
            if (CL(a)) {
                let c, d, e;
                b = (e = (d = (c = a.parts) != null ? c : a.partsList) != null ? d : a.parts_list) != null ? e : [];
                if (!Array.isArray(b))
                    return "";
                let f = b.map(g => {
                    if (!CL(g))
                        return {
                            text: "",
                            Kt: !1
                        };
                    var h = CL(g.reference) ? g.reference : void 0;
                    if (h) {
                        g = CL(h.media) ? h.media : void 0;
                        let k = CL(h.entity) ? h.entity : void 0
                          , l = CL(h.likeness) ? h.likeness : void 0;
                        h = CL(h.audio) ? h.audio : void 0;
                        return {
                            text: (typeof (g == null ? void 0 : g.handle) === "string" ? g.handle : void 0) || (typeof (k == null ? void 0 : k.handle) === "string" ? k.handle : void 0) || (typeof (l == null ? void 0 : l.handle) === "string" ? l.handle : void 0) || (typeof (h == null ? void 0 : h.handle) === "string" ? h.handle : void 0) || "",
                            Kt: !0
                        }
                    }
                    return {
                        text: typeof g.text === "string" ? g.text : "",
                        Kt: !1
                    }
                }
                ).filter(g => g.text !== "");
                a = f.reduce( (g, h, k) => {
                    if (k === 0)
                        return h.text;
                    k = f[k - 1];
                    if (!k)
                        return g + h.text;
                    k = k.Kt && (h.Kt || /^\w/.test(h.text)) || h.Kt && !k.text.endsWith(" ");
                    return g + (k ? " " : "") + h.text
                }
                , "");
                return _.wL(a)
            }
            return ""
        }
        ;
        _.QVa = function(a) {
            var b, c;
            return (c = (b = _.OVa(a)) == null ? void 0 : b.handle) != null ? c : ""
        }
        ;
        _.QL = function(a) {
            var b = _.Bw(a, _.FL);
            switch (b) {
            case 7:
                switch (a = _.GL(a),
                b = _.Bw(a, _.HL),
                b) {
                case 1:
                    return a = _.IL(a),
                    _.p(a, 14);
                case 2:
                    return a = _.hv(a, _.JL, 2, _.HL),
                    _.p(a, 4);
                case 0:
                    throw Error("Fe");
                default:
                    throw Error("Ge`" + b);
                }
            case 8:
                switch (a = _.KL(a),
                b = _.Bw(a, _.LL),
                b) {
                case 1:
                    return a = _.ML(a),
                    _.p(a, 9);
                case 5:
                    return _.NL(_.hv(a, _.RVa, 5, _.LL));
                case 0:
                    throw Error("He");
                default:
                    throw Error("Ie`" + b);
                }
            case 11:
                switch (a = _.OL(a),
                b = _.Bw(a, SVa),
                b) {
                case 1:
                    a = _.PL(a);
                    a = _.p(a, 4) || _.p(a, 6);
                    if (!a)
                        throw Error("Je");
                    return a;
                case 0:
                    throw Error("Ke");
                default:
                    throw Error("Le`" + b);
                }
            case 0:
                throw Error("Me");
            default:
                throw Error("Ne`" + b);
            }
        }
        ;
        _.SL = function(a) {
            if (a.Xo()) {
                var b = _.GL(a), c, d;
                let f;
                var e = (f = ((c = _.IL(b)) == null ? void 0 : c.Ve()) || ((d = _.hv(b, _.JL, 2, _.HL)) == null ? void 0 : d.Ve())) != null ? f : 0;
                let g, h;
                return (h = (g = _.WPa.get(e)) != null ? g : TVa(_.Y(b, _.RL, 3))) != null ? h : 1
            }
            if (_.vw(a, 8, _.FL)) {
                c = _.KL(a);
                d = (e = (b = _.ML(c)) == null ? void 0 : b.Ve()) != null ? e : 0;
                let f, g;
                return (g = (f = _.VPa.get(d)) != null ? f : TVa(_.Y(c, _.RL, 2))) != null ? g : 1
            }
        }
        ;
        TVa = function(a) {
            if (a && _.ou(a, 1) > 0 && _.ou(a, 2) > 0)
                return _.ou(a, 1) / _.ou(a, 2)
        }
        ;
        _.XL = function(a) {
            var b, c, d, e = (c = (b = _.TL(a)) == null ? void 0 : _.Y(b, _.UL, 7)) == null ? void 0 : (d = _.Dt(c, _.VL, 3)) == null ? void 0 : d[0];
            if (b = e == null ? void 0 : _.Y(e, _.WL, 3))
                if (b = _.EL(b))
                    return b;
            if (e = e == null ? void 0 : _.p(e, 1))
                return e;
            switch (_.Bw(a, _.FL)) {
            case 7:
                let f;
                return (f = _.IL(_.GL(a))) == null ? void 0 : _.p(f, 8);
            case 8:
                let g;
                return (g = _.ML(_.KL(a))) == null ? void 0 : _.p(g, 8)
            }
        }
        ;
        _.FL = [7, 8, 11];
        _.YL = class extends _.n {
            constructor(a) {
                super(a)
            }
            setAspectRatio(a) {
                return _.Uw(this, 2, a)
            }
        }
        ;
        _.YL.prototype.ZE = _.aa(30);
        _.YL.prototype.ZM = _.aa(28);
        _.YL.prototype.zt = _.aa(26);
        _.YL.prototype.Ve = _.aa(22);
        _.ZL = class extends _.n {
            constructor(a) {
                super(a)
            }
            setAspectRatio(a) {
                return _.Uw(this, 2, a)
            }
        }
        ;
        _.ZL.prototype.dF = _.aa(34);
        _.ZL.prototype.jN = _.aa(32);
        _.ZL.prototype.zt = _.aa(25);
        _.ZL.prototype.Ve = _.aa(21);
        _.$L = class extends _.n {
            constructor(a) {
                super(a)
            }
            setAspectRatio(a) {
                return _.Uw(this, 2, a)
            }
        }
        ;
        _.$L.prototype.ZE = _.aa(29);
        _.$L.prototype.ZM = _.aa(27);
        _.$L.prototype.zt = _.aa(24);
        _.$L.prototype.Ve = _.aa(20);
        _.aM = class extends _.n {
            constructor(a) {
                super(a)
            }
            setAspectRatio(a) {
                return _.Uw(this, 2, a)
            }
        }
        ;
        _.aM.prototype.dF = _.aa(33);
        _.aM.prototype.jN = _.aa(31);
        _.aM.prototype.zt = _.aa(23);
        _.aM.prototype.Ve = _.aa(19);
        var UVa = [2, 3, 4, 5, 10, 14];
        _.kz.prototype.Nr = _.ca(59, function() {
            return _.p(this, 4)
        });
        _.YL.prototype.Ve = _.ca(22, function() {
            return _.lu(this, 2)
        });
        _.ZL.prototype.Ve = _.ca(21, function() {
            return _.lu(this, 2)
        });
        _.$L.prototype.Ve = _.ca(20, function() {
            return _.lu(this, 2)
        });
        _.aM.prototype.Ve = _.ca(19, function() {
            return _.lu(this, 2)
        });
        _.bz.prototype.Xo = _.ca(13, function() {
            return _.vw(this, 7, _.FL)
        });
        _.Mz.prototype.Xo = _.ca(12, function() {
            return _.vw(this, 3, UVa)
        });
        _.bz.prototype.Be = _.ca(11, function() {
            return _.p(this, 3)
        });
        _.sz.prototype.Be = _.ca(10, function() {
            return _.p(this, 1)
        });
        _.vz.prototype.Be = _.ca(9, function() {
            return _.p(this, 1)
        });
        _.$y.prototype.Yh = _.ca(8, function() {
            return _.p(this, 2)
        });
        _.Fy.prototype.te = _.ca(4, function() {
            return _.p(this, 1)
        });
        _.cz.prototype.te = _.ca(3, function() {
            return _.p(this, 1)
        });
        _.kz.prototype.te = _.ca(2, function() {
            return _.p(this, 1)
        });
        _.Pz.prototype.te = _.ca(1, function() {
            return _.Ow(this, 2)
        });
        _.Sz.prototype.te = _.ca(0, function() {
            return _.Ow(this, 6)
        });
        _.zL = class extends _.n {
            constructor(a) {
                super(a)
            }
            Yh() {
                return _.p(this, 2)
            }
        }
        ;
        _.AL = class extends _.n {
            constructor(a) {
                super(a)
            }
            Mh() {
                return _.p(this, 1)
            }
            Yh() {
                return _.p(this, 2)
            }
        }
        ;
        _.BL = class extends _.n {
            constructor(a) {
                super(a)
            }
            Sk() {
                return _.p(this, 1)
            }
            Yh() {
                return _.p(this, 2)
            }
        }
        ;
        _.yL = class extends _.n {
            constructor(a) {
                super(a)
            }
            Yh() {
                return _.p(this, 2)
            }
        }
        ;
        _.xL = [1, 2, 3, 4];
        _.bM = class extends _.n {
            constructor(a) {
                super(a)
            }
            Lm() {
                return _.vw(this, 4, _.xL)
            }
        }
        ;
        _.bM.prototype.So = _.aa(122);
        _.bM.prototype.wi = _.aa(16);
        _.bM.prototype.We = _.aa(14);
        _.DL = [1, 2];
        _.cM = class extends _.n {
            constructor(a) {
                super(a)
            }
            Tn() {
                return _.Zr(this, 1, _.DL) != null
            }
            na() {
                return _.hv(this, _.bM, 2, _.DL)
            }
            ka() {
                return _.Fw(this, _.bM, 2, _.DL)
            }
            Ct() {
                return _.vw(this, 2, _.DL)
            }
        }
        ;
        _.cM.prototype.wr = _.aa(127);
        _.cM.prototype.ih = _.aa(104);
        _.WL = class extends _.n {
            constructor(a) {
                super(a)
            }
            na(a) {
                return _.Dt(this, _.cM, 1, a && _.xw(a))
            }
            ka() {
                return _.Ew(this, _.cM, 1)
            }
        }
        ;
        _.dM = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.dM.prototype.UL = _.aa(128);
        VVa = class extends _.n {
            constructor(a) {
                super(a)
            }
            getName() {
                return _.p(this, 1)
            }
            Ki() {
                return _.p(this, 2)
            }
        }
        ;
        SVa = [1];
        _.PL = function(a) {
            return _.hv(a, VVa, 1, SVa)
        }
        ;
        WVa = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.RL = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.VL = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.UL = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.eM = class extends _.n {
            constructor(a) {
                super(a)
            }
            Be() {
                return _.p(this, 12)
            }
            Ve() {
                return _.lu(this, 15)
            }
            setAspectRatio(a) {
                return _.Op(this, 15, a)
            }
        }
        ;
        _.eM.prototype.D4 = _.aa(129);
        _.JL = class extends _.n {
            constructor(a) {
                super(a)
            }
            Xo() {
                return _.mu(this, 1, void 0, void 0, _.Oc) != null
            }
            Ve() {
                return _.lu(this, 5)
            }
            setAspectRatio(a) {
                return _.Op(this, 5, a)
            }
        }
        ;
        _.JL.prototype.LU = _.aa(87);
        _.HL = [1, 2];
        _.IL = function(a) {
            return _.hv(a, _.eM, 1, _.HL)
        }
        ;
        _.XVa = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.fM = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.fM.prototype.Uk = _.aa(130);
        _.YVa = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ve() {
                return _.lu(this, 17)
            }
            setAspectRatio(a) {
                return _.Op(this, 17, a)
            }
        }
        ;
        _.NL = function(a) {
            return _.p(a, 3)
        }
        ;
        _.RVa = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.LL = [1, 5];
        _.ML = function(a) {
            return _.hv(a, _.YVa, 1, _.LL)
        }
        ;
        _.ZVa = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.TL = function(a) {
            return _.Y(a, _.fM, 6)
        }
        ;
        _.GL = function(a) {
            return _.hv(a, _.XVa, 7, _.FL)
        }
        ;
        _.KL = function(a) {
            return _.hv(a, _.ZVa, 8, _.FL)
        }
        ;
        _.OL = function(a) {
            return _.hv(a, WVa, 11, _.FL)
        }
        ;
        _.gM = function(a) {
            return a.Yb.Vg && a.ha.Ia()
        }
        ;
        _.hM = function(a, b, c) {
            _.du.Dg( () => a.queryCache.findAll(b).map( ({queryKey: d}) => [d, _.YH(a, d, c)]))
        }
        ;
        $Va = function() {
            var a = _.DJ();
            return _.CJ(a, {
                kind: "min",
                value: 0,
                Ng: !1,
                message: _.rJ.toString(void 0)
            })
        }
        ;
        _.iM = class extends _.n {
            constructor(a) {
                super(a)
            }
            te() {
                return _.p(this, 2)
            }
        }
        ;
        _.jM = function(a) {
            return _.Dw(a, _.iM, 4)
        }
        ;
        _.kM = class extends _.n {
            constructor(a) {
                super(a)
            }
            lc() {
                return _.p(this, 1)
            }
            Mh() {
                return _.p(this, 2)
            }
            Nr() {
                return _.p(this, 9)
            }
        }
        ;
        _.kM.prototype.ha = _.aa(81);
        _.kM.prototype.DB = _.aa(5);
        _.aWa = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ib() {
                return _.Y(this, _.Z, 2)
            }
            Hb() {
                return _.kv(this, _.Z, 2)
            }
            getName() {
                return _.p(this, 1)
            }
        }
        ;
        _.bWa = {
            Xa: "as29s",
            Za: "/FlowService.GetMedia",
            hb: !0,
            Ya: _.bz,
            lb: _.aWa
        };
        _.bWa[_.Hy] = !0;
        _.cWa = class extends _.wJ {
            Vf(a) {
                return a.data !== this.Xb.value ? (a = this.ka(a),
                _.Gu(a, {
                    ml: a.data,
                    code: _.lJ.hJa,
                    Nn: this.Xb.value
                }),
                _.nJ) : {
                    status: "valid",
                    value: a.data
                }
            }
            get value() {
                return this.Xb.value
            }
        }
        ;
        _.lM = a => new _.cWa(Object.assign({}, {
            value: a,
            typeName: "ZodLiteral"
        }, _.Hu()));
        dWa = class extends _.wJ {
            constructor() {
                super(...arguments);
                this.wa = !0
            }
            Vf(a) {
                return _.pJ(a.data)
            }
        }
        ;
        _.mM = () => new dWa(Object.assign({}, {
            typeName: "ZodAny"
        }, _.Hu()));
        fWa = function(a, b) {
            return new eWa(Object.assign({}, a.Xb, {
                Hi: [...a.Xb.Hi, b]
            }))
        }
        ;
        eWa = class extends _.wJ {
            Vf(a) {
                this.Xb.SC && (a.data = new Date(a.data));
                if (this.oa(a) !== _.Ju.date)
                    return a = this.ka(a),
                    _.Gu(a, {
                        code: _.lJ.Qr,
                        Nn: _.Ju.date,
                        ml: a.Gj
                    }),
                    _.nJ;
                if (Number.isNaN(a.data.getTime()))
                    return a = this.ka(a),
                    _.Gu(a, {
                        code: _.lJ.fJa
                    }),
                    _.nJ;
                var b = new _.oJ
                  , c = void 0;
                for (let d of this.Xb.Hi)
                    d.kind === "min" ? a.data.getTime() < d.value && (c = this.ka(a, c),
                    _.Gu(c, {
                        code: _.lJ.iK,
                        message: d.message,
                        Ng: !0,
                        exact: !1,
                        Dv: d.value,
                        type: "date"
                    }),
                    b.Wb()) : d.kind === "max" ? a.data.getTime() > d.value && (c = this.ka(a, c),
                    _.Gu(c, {
                        code: _.lJ.hK,
                        message: d.message,
                        Ng: !0,
                        exact: !1,
                        maximum: d.value,
                        type: "date"
                    }),
                    b.Wb()) : _.Ku.oG();
                return {
                    status: b.value,
                    value: new Date(a.data.getTime())
                }
            }
            min(a, b) {
                return fWa(this, {
                    kind: "min",
                    value: a.getTime(),
                    message: _.rJ.toString(b)
                })
            }
            max(a, b) {
                return fWa(this, {
                    kind: "max",
                    value: a.getTime(),
                    message: _.rJ.toString(b)
                })
            }
        }
        ;
        gWa = () => new eWa(Object.assign({}, {
            Hi: [],
            SC: !1,
            typeName: "ZodDate"
        }, _.Hu()));
        hWa = class extends _.wJ {
            Vf(a) {
                this.Xb.SC && (a.data = !!a.data);
                return this.oa(a) !== _.Ju.tfa ? (a = this.ka(a),
                _.Gu(a, {
                    code: _.lJ.Qr,
                    Nn: _.Ju.tfa,
                    ml: a.Gj
                }),
                _.nJ) : _.pJ(a.data)
            }
        }
        ;
        _.nM = () => new hWa(Object.assign({}, {
            typeName: "ZodBoolean",
            SC: !1
        }, _.Hu()));
        _.oM = {
            Ci: "LANDSCAPE",
            Yq: "LANDSCAPE_4_3",
            Fo: "SQUARE",
            br: "PORTRAIT_3_4",
            rk: "PORTRAIT"
        };
        _.pM = {
            [_.oM.Ci]: {
                label: "16:9",
                icon: "crop_16_9"
            },
            [_.oM.Yq]: {
                label: "4:3",
                icon: "crop_landscape"
            },
            [_.oM.Fo]: {
                label: "1:1",
                icon: "crop_square"
            },
            [_.oM.br]: {
                label: "3:4",
                icon: "crop_portrait"
            },
            [_.oM.rk]: {
                label: "9:16",
                icon: "crop_9_16"
            }
        };
        _.qM = {
            od: "IMAGE",
            qk: "LIKENESS",
            Ze: "CHARACTER",
            ne: "VIDEO",
            Ti: "AUDIO"
        };
        _.iWa = Object.values(_.qM);
        _.HJ({
            width: _.DJ(),
            height: _.DJ()
        });
        _.rL = Symbol("se");
        _.tL = Symbol("te");
        _.rM = Symbol("ue");
        _.sM = Symbol("ve");
        var jWa = _.HJ({
            workflowId: _.AJ(),
            ke: _.AJ(),
            nb: _.AJ(),
            position: _.DJ(),
            Nc: _.DJ(),
            Sg: _.DJ(),
            Fg: _.DJ()
        });
        _.tM = {
            [_.oM.Ci]: _.cI,
            [_.oM.Yq]: _.UPa,
            [_.oM.Fo]: 1,
            [_.oM.br]: .75,
            [_.oM.rk]: .5625
        };
        _.kWa = {
            "1:1": _.oM.Fo,
            "16:9": _.oM.Ci,
            "9:16": _.oM.rk,
            "4:3": _.oM.Yq,
            "3:4": _.oM.br,
            SQUARE: _.oM.Fo,
            LANDSCAPE: _.oM.Ci,
            PORTRAIT: _.oM.rk,
            IMAGE_ASPECT_RATIO_SQUARE: _.oM.Fo,
            IMAGE_ASPECT_RATIO_LANDSCAPE: _.oM.Ci,
            IMAGE_ASPECT_RATIO_PORTRAIT: _.oM.rk,
            IMAGE_ASPECT_RATIO_LANDSCAPE_FOUR_THREE: _.oM.Yq,
            IMAGE_ASPECT_RATIO_PORTRAIT_THREE_FOUR: _.oM.br,
            VIDEO_ASPECT_RATIO_LANDSCAPE: _.oM.Ci,
            VIDEO_ASPECT_RATIO_PORTRAIT: _.oM.rk
        };
        _.lWa = new _.dv("{DURATION} gi\u00e2y");
        _.mWa = new _.dv("{MINUTES} ph\u00fat {SECONDS} gi\u00e2y");
        _.nWa = new _.dv("{MINUTES} ph\u00fat");
        _.oWa = new _.dv("{DURATION} gi\u00e2y");
        var PVa = _.oq(_.WL);
        _.pWa = new Map([[20, 0], [6, 1], [18, 2], [19, 3], [14, 4]]);
        _.qWa = new Map([[3, 0], [7, 1]]);
        var uM, sWa;
        uM = _.HJ({
            id: _.AJ().min(1),
            displayName: _.AJ(),
            Qb: gWa(),
            Sb: gWa().optional(),
            yb: _.nM(),
            rb: _.nM(),
            oc: _.AJ().optional()
        });
        _.rWa = uM.extend({
            type: _.lM("SCENE"),
            me: _.AJ().optional(),
            aspectRatio: $Va().optional(),
            Lc: _.vJ(jWa).default([]),
            xf: _.mM().default(new Map),
            isPending: _.nM().optional()
        });
        sWa = uM.extend({
            nb: _.AJ().optional(),
            workflowId: _.AJ().optional(),
            Cd: _.AJ().optional()
        });
        _.tWa = sWa.extend({
            type: _.lM("VIDEO"),
            Cb: _.mM(),
            history: _.mM(),
            yo: _.mM().optional()
        });
        _.uWa = sWa.extend({
            type: _.lM("IMAGE"),
            Cb: _.mM(),
            history: _.mM()
        });
        _.vWa = uM.extend({
            type: _.lM("COLLECTION"),
            me: _.AJ().optional(),
            isPending: _.nM().optional()
        });
        _.wWa = uM.extend({
            type: _.lM("CHARACTER"),
            me: _.AJ().optional(),
            aspectRatio: $Va().optional(),
            jd: _.mM().optional(),
            Bi: _.mM().default(new Map)
        });
        _.vM = class {
            constructor() {
                this.Da = _.r(_.NH);
                _.r(_.uK);
                this.ma = _.r(_.OF);
                this.Bb = _.r(_.bu);
                this.rl = _.Ft({
                    Bb: this.Bb
                });
                this.ha = _.NVa({
                    isEnabled: () => this.Da.Yb.Nb,
                    Aja: () => this.Da.Yb.Sa,
                    Cja: () => this.Da.Yb.Va,
                    sna: 3E4
                });
                this.ka = _.P(new Set)
            }
        }
        ;
        _.vM.va = function(a) {
            return new (a || _.vM)
        }
        ;
        _.vM.kb = _.mg({
            token: _.vM,
            Qa: _.vM.va
        });
        var $Wa, aXa, bXa, LM, cXa, fXa, gXa, hXa, lXa, rXa, sXa, dXa;
        $Wa = function(a, b, c=0) {
            a = [...a, b];
            return c && a.length > c ? a.slice(1) : a
        }
        ;
        aXa = function(a, b, c=0) {
            a = [b, ...a];
            return c && a.length > c ? a.slice(0, -1) : a
        }
        ;
        bXa = function(a, {pages: b, pageParams: c}) {
            var d;
            return b.length > 0 ? (d = a.xHa) == null ? void 0 : d.call(a, b[0], b, c[0], c) : void 0
        }
        ;
        LM = function(a, {pages: b, pageParams: c}) {
            var d = b.length - 1;
            return b.length > 0 ? a.getNextPageParam(b[d], b, c[d], c) : void 0
        }
        ;
        cXa = function() {
            return {
                onFetch: (a, b) => {
                    var c = a.options, d, e, f, g = (d = a.fetchOptions) == null ? void 0 : (e = d.meta) == null ? void 0 : (f = e.fetchMore) == null ? void 0 : f.direction, h, k = ((h = a.state.data) == null ? void 0 : h.pages) || [], l, q = ((l = a.state.data) == null ? void 0 : l.pageParams) || [], u = {
                        pages: [],
                        pageParams: []
                    }, x = 0, y = () => _.A(function*() {
                        var C = !1
                          , G = ba => {
                            Object.defineProperty(ba, "signal", {
                                enumerable: !0,
                                get: () => {
                                    a.signal.aborted ? C = !0 : a.signal.addEventListener("abort", () => {
                                        C = !0
                                    }
                                    );
                                    return a.signal
                                }
                            })
                        }
                          , J = _.nqa(a.options, a.fetchOptions)
                          , N = (ba, ha, Ea) => _.A(function*() {
                            if (C)
                                return Promise.reject();
                            if (ha == null && ba.pages.length)
                                return Promise.resolve(ba);
                            var Sa = {
                                client: a.client,
                                queryKey: a.queryKey,
                                n8: ha,
                                direction: Ea ? "backward" : "forward",
                                meta: a.options.meta
                            };
                            G(Sa);
                            Sa = yield J(Sa);
                            var gb = a.options.maxPages
                              , cb = Ea ? aXa : $Wa;
                            return {
                                pages: cb(ba.pages, Sa, gb),
                                pageParams: cb(ba.pageParams, ha, gb)
                            }
                        });
                        if (g && k.length) {
                            var W = g === "backward"
                              , U = {
                                pages: k,
                                pageParams: q
                            }
                              , da = (W ? bXa : LM)(c, U);
                            u = yield N(U, da, W)
                        } else {
                            W = k.length;
                            do {
                                U = void 0;
                                da = x === 0 ? (U = q[0]) != null ? U : c.initialPageParam : LM(c, u);
                                if (x > 0 && da == null)
                                    break;
                                u = yield N(u, da);
                                x++
                            } while (x < W)
                        }
                        return u
                    });
                    a.fetchFn = a.options.persister ? () => {
                        var C, G;
                        return (G = (C = a.options).persister) == null ? void 0 : G.call(C, y, {
                            client: a.client,
                            queryKey: a.queryKey,
                            meta: a.options.meta,
                            signal: a.signal
                        }, b)
                    }
                    : y
                }
            }
        }
        ;
        _.MM = function(a) {
            return _.eu( () => _.wqa(a, dXa))
        }
        ;
        _.OM = function(a, b, c) {
            _.hM(a, {
                queryKey: ["PROJECTS"]
            }, d => d ? Object.assign({}, d, {
                pages: d.pages.map(e => Object.assign({}, e, {
                    Bd: e.Bd.map(f => {
                        if (f.lc() !== b)
                            return f;
                        f = _.dc(f);
                        var g = f.getMetadata();
                        g || (g = new _.kz,
                        _.Lp(f, 2, g));
                        c.Kj !== void 0 && _.NM(g, c.Kj);
                        c.ew !== void 0 && (g = _.Rw(g, 5, c.ew),
                        _.Rw(g, 4, ""));
                        return _.Cc(f)
                    }
                    )
                }))
            }) : d)
        }
        ;
        _.PM = function(a) {
            var b = a.ya;
            _.hM(a.Bb, {
                queryKey: ["PROJECTS"],
                predicate: c => {
                    c = c.queryKey[1];
                    return c === void 0 || c === 3
                }
            }, c => {
                if (!c)
                    return c;
                var d = c.pages.flatMap(f => f.Bd)
                  , e = d.find(f => f.lc() === b);
                return e && d[0] !== e ? Object.assign({}, c, {
                    pages: c.pages.map( (f, g) => {
                        var h = f.Bd.filter(k => k.lc() !== b);
                        return Object.assign({}, f, {
                            Bd: g === 0 ? [e, ...h] : h
                        })
                    }
                    )
                }) : c
            }
            )
        }
        ;
        fXa = function(a) {
            var b = a.iA();
            var c = _.Vu(eXa());
            var d = a.lc();
            c = _.Rw(c, 1, d);
            d = _.NM(new _.kz, b == null ? void 0 : _.p(b, 1));
            b = b == null ? void 0 : _.QM(b);
            b = _.Rw(d, 5, b);
            a = _.Y(a, _.Ay, 3);
            a = _.Lp(b, 3, a);
            a = _.Cc(a);
            c = _.Lp(c, 2, a);
            return _.Cc(c)
        }
        ;
        _.RM = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        _.SM = class extends _.n {
            constructor(a) {
                super(a)
            }
            lc() {
                return _.p(this, 1)
            }
            Vk() {
                return _.Dt(this, _.vz, 4)
            }
        }
        ;
        _.SM.prototype.iA = _.aa(83);
        _.SM.prototype.Tk = _.aa(65);
        _.SM.prototype.mx = _.aa(60);
        _.SM.prototype.iA = _.ca(83, function() {
            return _.Y(this, _.RM, 2)
        });
        _.TM = function(a) {
            return _.Op(a, 2, 22)
        }
        ;
        _.UM = class extends _.n {
            constructor(a) {
                super(a)
            }
            Be() {
                return _.p(this, 5)
            }
            lc() {
                return _.p(this, 6)
            }
            zH() {
                return _.lu(this, 7)
            }
        }
        ;
        gXa = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
        ;
        hXa = _.Ed(gXa);
        _.VM = function() {
            var a = _.Vu(hXa());
            return _.Uw(a, 1, 1)
        }
        ;
        _.NM = function(a, b) {
            return _.Rw(a, 1, b)
        }
        ;
        _.iXa = function(a) {
            var b = (new _.pz).jn("projects/*");
            return _.Pw(b, 2, a)
        }
        ;
        _.jXa = function(a, b) {
            return _.Tw(a, 3, b)
        }
        ;
        _.QM = function(a) {
            return _.p(a, 2)
        }
        ;
        _.WM = function(a, b) {
            var c = a.queryCache;
            _.du.Dg( () => {
                c.findAll(b).forEach(d => {
                    c.remove(d)
                }
                )
            }
            )
        }
        ;
        _.XM = function(a, b) {
            a.ka.update(c => {
                c = new Set(c);
                for (let d of b)
                    try {
                        if (d && d.includes("storage.googleapis.com"))
                            throw Error("Ue`" + d);
                        c.add(d)
                    } catch (e) {
                        _.At(e, "warning")
                    }
                return c
            }
            )
        }
        ;
        _.YM = class extends _.n {
            constructor(a) {
                super(a)
            }
            lc() {
                return _.p(this, 1)
            }
            getMetadata() {
                return _.Y(this, _.kz, 2)
            }
        }
        ;
        _.YM.prototype.cA = _.aa(132);
        var eXa = _.Ed(_.YM)
          , kXa = {
            Xa: "UpteDb",
            Za: "/FlowService.GetProjects",
            hb: !0,
            Ya: class extends _.n {
                constructor(a) {
                    super(a)
                }
            }
            ,
            lb: _.pz
        };
        kXa[_.Hy] = !0;
        _.ZM = new _.Yy(kXa);
        lXa = {
            Xa: "jHPbke",
            Za: "/AiSandbox.CreateProject",
            hb: !1,
            Ya: _.SM,
            lb: _.Kz
        };
        lXa[_.Hy] = !0;
        var mXa = new _.Yy(lXa)
          , nXa = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ib() {
                return _.Y(this, _.Z, 2)
            }
            Hb() {
                return _.kv(this, _.Z, 2)
            }
            getName() {
                return _.p(this, 1)
            }
        }
          , oXa = {
            Xa: "QI2zvc",
            Za: "/AiSandbox.DeleteProject",
            hb: !1,
            Ya: _.gz,
            lb: nXa
        };
        oXa[_.Hy] = !0;
        var pXa = new _.Yy(oXa)
          , qXa = {
            Xa: "OylIJd",
            Za: "/AiSandbox.SearchUserProjects",
            hb: !0,
            Ya: class extends _.n {
                constructor(a) {
                    super(a)
                }
            }
            ,
            lb: _.Nz
        };
        qXa[_.Hy] = !0;
        rXa = new _.Yy(qXa);
        _.$M = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ib() {
                return _.Y(this, _.Z, 5)
            }
            Hb() {
                return _.kv(this, _.Z, 5)
            }
            getName() {
                return _.p(this, 1)
            }
            iA() {
                return _.Y(this, _.RM, 2)
            }
        }
        ;
        sXa = {
            Xa: "o8DA4",
            Za: "/AiSandbox.UpdateProjectInfo",
            hb: !1,
            Ya: _.RM,
            lb: _.$M
        };
        sXa[_.Hy] = !0;
        _.aN = new _.Yy(sXa);
        dXa = class extends _.gu {
            bindMethods() {
                super.bindMethods();
                this.fetchNextPage = this.fetchNextPage.bind(this);
                this.fetchPreviousPage = this.fetchPreviousPage.bind(this)
            }
            setOptions(a) {
                super.setOptions(Object.assign({}, a, {
                    behavior: cXa()
                }))
            }
            getOptimisticResult(a) {
                a.behavior = cXa();
                return super.getOptimisticResult(a)
            }
            fetchNextPage(a) {
                return this.fetch(Object.assign({}, a, {
                    meta: {
                        fetchMore: {
                            direction: "forward"
                        }
                    }
                }))
            }
            fetchPreviousPage(a) {
                return this.fetch(Object.assign({}, a, {
                    meta: {
                        fetchMore: {
                            direction: "backward"
                        }
                    }
                }))
            }
            createResult(a, b) {
                var c = a.state;
                a = super.createResult(a, b);
                var d = a.isFetching, e = a.isRefetching, f = a.isError, g = a.isRefetchError, h, k, l = (h = c.fetchMeta) == null ? void 0 : (k = h.fetchMore) == null ? void 0 : k.direction;
                h = f && l === "forward";
                k = d && l === "forward";
                f = f && l === "backward";
                d = d && l === "backward";
                l = (l = c.data) ? LM(b, l) != null : !1;
                var q = Object
                  , u = q.assign
                  , x = this.fetchNextPage
                  , y = this.fetchPreviousPage;
                b = (c = c.data) && b.xHa ? bXa(b, c) != null : !1;
                return u.call(q, {}, a, {
                    fetchNextPage: x,
                    fetchPreviousPage: y,
                    hasNextPage: l,
                    hasPreviousPage: b,
                    isFetchNextPageError: h,
                    isFetchingNextPage: k,
                    isFetchPreviousPageError: f,
                    isFetchingPreviousPage: d,
                    isRefetchError: g && !h && !f,
                    isRefetching: e && !k && !d
                })
            }
        }
        ;
        _.bN = class {
            constructor() {
                this.Uc = _.r(_.XI, {
                    optional: !0
                });
                this.Kb = _.r(_.vh);
                this.ha = _.r(_.$I);
                this.type = "button"
            }
            ho() {
                this.Uc || (this.Uc = _.Zqa(this.Kb, this.ha.ha))
            }
            Sl(a) {
                if (a = a._matDialogClose)
                    this.iU = a.ka
            }
            AS(a) {
                if (this.Kb.Ja.getAttribute("aria-disabled") !== "true") {
                    var b = this.Uc
                      , c = this.iU;
                    b.QF = a.screenX === 0 && a.screenY === 0 ? "keyboard" : "mouse";
                    b.close(c)
                }
            }
        }
        ;
        _.bN.va = function(a) {
            return new (a || _.bN)
        }
        ;
        _.bN.nd = _.sg({
            type: _.bN,
            Fa: [["", "mat-dialog-close", ""], ["", "matDialogClose", ""]],
            nc: 2,
            Ob: function(a, b) {
                a & 1 && _.M("click", function(c) {
                    return b.AS(c)
                });
                a & 2 && _.bk("aria-label", b.ariaLabel || null)("type", b.type)
            },
            inputs: {
                ariaLabel: [0, "aria-label", "ariaLabel"],
                type: "type",
                iU: [0, "mat-dialog-close", "dialogResult"],
                OWa: [0, "matDialogClose", "_matDialogClose"]
            },
            Rd: ["matDialogClose"],
            features: [_.Gx]
        });
        _.tXa = class {
            constructor(a) {
                this.ha = a;
                this.active = 0;
                this.queue = []
            }
            run(a) {
                var b = this;
                return _.A(function*() {
                    b.active >= b.ha ? yield new Promise(c => {
                        b.queue.push(c)
                    }
                    ) : b.active++;
                    try {
                        return yield a()
                    } finally {
                        if (b.queue.length > 0) {
                            let c;
                            (c = b.queue.shift()) == null || c()
                        } else
                            b.active--
                    }
                })
            }
        }
        ;
        var uXa, xXa;
        _.cN = _.Wu(_.TM(new _.UM));
        _.dN = ["PROJECTS"];
        uXa = function(a, b) {
            return _.A(function*() {
                var c = a.ha
                  , d = c.mutate
                  , e = mXa.Eb
                  , f = (new _.Kz).jn("projects/*")
                  , g = new _.SM;
                var h = new _.RM;
                h = _.Rw(h, 1, b);
                h = _.Cc(h);
                g = _.Lp(g, 2, h);
                g = _.Cc(g);
                f = _.Lp(f, 2, g);
                f = _.Lp(f, 3, _.cN);
                return yield d.call(c, e.call(mXa, _.Cc(f)))
            })
        }
        ;
        _.vXa = function(a, b, c=!1) {
            return _.A(function*() {
                var d = pXa.Eb;
                var e = new nXa;
                e = _.Rw(e, 1, `projects/${b}`);
                d = d.call(pXa, _.Cc(e));
                c && (d = d.oj(_.Ar, _.Fr()));
                yield a.ha.mutate(d)
            })
        }
        ;
        _.eN = function(a) {
            return _.fu( () => ({
                enabled: !0,
                mutationKey: ["CREATE_PROJECT"],
                mutationFn: b => uXa(a, b),
                onSuccess: () => {
                    _.$H(a.Bb, {
                        queryKey: _.dN
                    })
                }
            }))
        }
        ;
        xXa = function(a, b) {
            _.wXa(a, c => c.filter(d => d.lc() !== b))
        }
        ;
        _.yXa = function(a, {lc: b, lD: c}) {
            return _.fu( () => ({
                enabled: !!b(),
                mutationKey: ["DELETE_PROJECT", b()],
                mutationFn: () => _.vXa(a, b()),
                onMutate: () => _.A(function*() {
                    if (c) {
                        var d = b();
                        yield Promise.allSettled([_.ZH(a.Bb, {
                            queryKey: _.dN
                        }), ...(d ? [_.ZH(a.Bb, {
                            queryKey: ["PROJECT", d]
                        })] : [])]);
                        d && xXa(a, d)
                    }
                }),
                onSuccess: () => {
                    if (!c) {
                        let d = b();
                        d && xXa(a, d)
                    }
                    _.WM(a.Bb, {
                        queryKey: ["PROJECT", b()]
                    })
                }
                ,
                onError: () => {
                    c && _.$H(a.Bb, {
                        queryKey: _.dN
                    })
                }
            }))
        }
        ;
        _.wXa = function(a, b) {
            _.hM(a.Bb, {
                queryKey: _.dN
            }, c => c ? Object.assign({}, c, {
                pages: c.pages.map(d => Object.assign({}, d, {
                    Bd: b(d.Bd)
                }))
            }) : c)
        }
        ;
        _.fN = class {
            constructor() {
                this.ha = _.r(_.OF);
                this.Bb = _.r(_.bu);
                this.Da = _.r(_.NH);
                this.ma = _.r(_.IH);
                this.Cc = _.r(_.vM);
                _.r(_.wy);
                this.ka = this.ma.signal(_.HH.Dya, 3);
                this.Bs = _.X( () => this.ka() === 1 ? 1 : 3)
            }
            get ie() {
                return _.st()
            }
            Zb(a) {
                return _.MM( () => {
                    var b = this.Da.Yb.ha && !this.Da.Yb.Aa ? this.Bs() : void 0, c, d;
                    return {
                        queryKey: b !== void 0 ? ["PROJECTS", b] : _.dN,
                        enabled: (d = (c = typeof a === "function" ? a() : a) == null ? void 0 : c.enabled) != null ? d : this.ie,
                        queryFn: ({n8: e}) => {
                            var f = this;
                            return _.A(function*() {
                                if (f.Da.Yb.Aa) {
                                    var g = f.ha
                                      , h = g.fetch
                                      , k = rXa.Eb;
                                    var l = (new _.Nz).jn("projects/*");
                                    l = _.Pw(l, 2, 20);
                                    l = _.Tw(l, 3, e);
                                    l = _.Lp(l, 4, _.cN);
                                    g = yield h.call(g, k.call(rXa, _.Cc(l)));
                                    return {
                                        Bd: _.Dt(g, _.SM, 1).map(fXa),
                                        MI: _.wc(_.yw(g, 2))
                                    }
                                }
                                var q = _.gM(f.Da) ? _.Wu(_.VM()) : void 0;
                                k = f.ha;
                                l = k.fetch;
                                var u = _.ZM.Eb
                                  , x = _.jXa(_.iXa(21), e);
                                q = _.Lp(x, 7, q);
                                q = _.Uw(q, 4, b);
                                k = yield l.call(k, u.call(_.ZM, _.Cc(q)));
                                u = _.Dt(k, _.YM, 1);
                                if (_.gM(f.Da)) {
                                    l = [];
                                    for (g of u)
                                        h = void 0,
                                        (u = (h = g.getMetadata()) == null ? void 0 : h.Nr()) && l.push(u);
                                    l.length > 0 && _.XM(f.Cc, l)
                                }
                                return {
                                    Bd: _.Dt(k, _.YM, 1),
                                    MI: _.wc(_.yw(k, 2))
                                }
                            })
                        }
                        ,
                        initialPageParam: "",
                        getNextPageParam: e => e.MI || void 0,
                        placeholderData: _.Rt,
                        refetchOnMount: !0,
                        staleTime: 36E5
                    }
                }
                )
            }
        }
        ;
        _.fN.prototype.dU = _.aa(133);
        _.fN.va = function(a) {
            return new (a || _.fN)
        }
        ;
        _.fN.kb = _.mg({
            token: _.fN,
            Qa: _.fN.va
        });
        var ZYa, $Ya, aZa;
        ZYa = function(a) {
            a & 1 && (_.I(0, "div", 0)(1, "mat-icon", 7),
            _.R(2),
            _.K()());
            a & 2 && (a = _.O(),
            _.B(2),
            _.S(a.data.icon))
        }
        ;
        $Ya = function(a) {
            a & 1 && (_.I(0, "h2", 1),
            _.R(1),
            _.K());
            a & 2 && (a = _.O(),
            _.B(),
            _.S(a.data.title))
        }
        ;
        aZa = function(a) {
            a & 1 && _.L(0, "mat-spinner", 6);
            a & 2 && _.H("diameter", 18)
        }
        ;
        _.ZN = class {
            constructor() {
                this.Uc = _.r(_.XI);
                this.data = _.r(_.YI);
                var a;
                this.Mk = (a = this.data.Mk) != null ? a : "X\u00e1c nh\u1eadn";
                var b;
                this.Tj = (b = this.data.Tj) != null ? b : "Hu\u1ef7";
                var c;
                this.eq = (c = this.data.eq) != null ? c : !1;
                this.isLoading = _.P(!1);
                this.rO = !this.data.title
            }
            K2() {
                var a = this;
                return _.A(function*() {
                    if (a.data.Aq) {
                        a.isLoading.set(!0);
                        a.Uc.Lg = !0;
                        try {
                            yield a.data.Aq(),
                            a.isLoading.set(!1),
                            a.Uc.close(!0)
                        } catch (b) {
                            a.isLoading.set(!1),
                            a.Uc.Lg = !1
                        }
                    } else
                        a.Uc.close(!0)
                })
            }
            NC() {
                this.Uc.close(!1)
            }
        }
        ;
        _.ZN.va = function(a) {
            return new (a || _.ZN)
        }
        ;
        _.ZN.Ma = _.z({
            type: _.ZN,
            Fa: [["flow-confirmation-dialog"]],
            Ka: 13,
            La: 23,
            Na: [[1, "dialog-icon-container"], ["mat-dialog-title", ""], [1, "dialog-message"], [3, "align"], ["flow-button", "", "matButton", "", 3, "click", "variant", "disabled"], [1, "confirm-button-content"], [3, "diameter"], [1, "flow-icon-l"]],
            template: function(a, b) {
                a & 1 && (_.E(0, ZYa, 3, 1, "div", 0),
                _.E(1, $Ya, 2, 1, "h2", 1),
                _.I(2, "mat-dialog-content")(3, "p", 2),
                _.R(4),
                _.K()(),
                _.I(5, "mat-dialog-actions", 3)(6, "button", 4),
                _.M("click", function() {
                    return b.NC()
                }),
                _.R(7),
                _.K(),
                _.I(8, "button", 4),
                _.M("click", function() {
                    return b.K2()
                }),
                _.I(9, "div", 5),
                _.E(10, aZa, 1, 1, "mat-spinner", 6),
                _.I(11, "span"),
                _.R(12),
                _.K()()()());
                a & 2 && (_.F(b.data.icon ? 0 : -1),
                _.B(),
                _.F(b.data.title ? 1 : -1),
                _.B(),
                _.Q("no-title-content", b.rO),
                _.B(),
                _.Q("centered-message", b.rO),
                _.B(),
                _.S(b.data.message),
                _.B(),
                _.Q("full-width-actions", b.rO),
                _.H("align", "end"),
                _.B(),
                _.Q("full-width-action-button", b.rO),
                _.H("variant", b.eq ? "secondary" : "primary")("disabled", b.isLoading()),
                _.bk("cdkFocusInitial", b.eq ? "" : null),
                _.B(),
                _.il(" ", b.Tj, " "),
                _.B(),
                _.Q("full-width-action-button", b.rO),
                _.H("variant", b.eq ? "primary" : "secondary")("disabled", b.isLoading()),
                _.bk("cdkFocusInitial", b.eq ? null : ""),
                _.B(2),
                _.F(b.isLoading() ? 10 : -1),
                _.B(2),
                _.S(b.Mk))
            },
            dependencies: [_.EH, _.BH, _.dJ, _.cJ, _.bJ, _.jI, _.yI],
            styles: [".confirm-button-content[_ngcontent-%COMP%]{display:-webkit-inline-box;display:-webkit-inline-flex;display:-moz-inline-box;display:-ms-inline-flexbox;display:inline-flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.5rem}.dialog-icon-container[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-pack:center;-webkit-justify-content:center;-moz-box-pack:center;-ms-flex-pack:center;justify-content:center;padding-top:1rem;padding-bottom:.75rem}.dialog-message[_ngcontent-%COMP%]{white-space:pre-line}.centered-message[_ngcontent-%COMP%]{text-align:center}mat-dialog-content.no-title-content[_ngcontent-%COMP%]{padding:0 1rem}mat-dialog-actions.full-width-actions[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;width:100%;-moz-box-sizing:border-box;box-sizing:border-box;gap:.5rem;padding:1rem}mat-dialog-actions.full-width-actions[_ngcontent-%COMP%]   button.full-width-action-button[_ngcontent-%COMP%]{-webkit-box-flex:1;-webkit-flex:1;-moz-box-flex:1;-ms-flex:1;flex:1;width:100%;margin-inline:0;white-space:nowrap}\n/*# sourceMappingURL=confirmation_dialog.css.map */"]
        });
        _.aO = class {
            constructor() {
                this.ka = this.ha = !1
            }
            get vertical() {
                return this.ha
            }
            set vertical(a) {
                this.ha = _.Hs(a)
            }
            get inset() {
                return this.ka
            }
            set inset(a) {
                this.ka = _.Hs(a)
            }
        }
        ;
        _.aO.va = function(a) {
            return new (a || _.aO)
        }
        ;
        _.aO.Ma = _.z({
            type: _.aO,
            Fa: [["mat-divider"]],
            Bc: ["role", "separator", 1, "mat-divider"],
            nc: 7,
            Ob: function(a, b) {
                a & 2 && (_.bk("aria-orientation", b.vertical ? "vertical" : "horizontal"),
                _.Q("mat-divider-vertical", b.vertical)("mat-divider-horizontal", !b.vertical)("mat-divider-inset", b.inset))
            },
            inputs: {
                vertical: "vertical",
                inset: "inset"
            },
            Ka: 0,
            La: 0,
            template: function() {},
            styles: [".mat-divider{display:block;margin:0;border-top-style:solid;border-top-color:var(--%NS%mat-divider-color, var(--%NS%mat-sys-outline-variant));border-top-width:var(--%NS%mat-divider-width, 1px)}.mat-divider.mat-divider-vertical{border-top:0;border-right-style:solid;border-right-color:var(--%NS%mat-divider-color, var(--%NS%mat-sys-outline-variant));border-right-width:var(--%NS%mat-divider-width, 1px)}.mat-divider.mat-divider-inset{margin-left:80px}[dir=rtl] .mat-divider.mat-divider-inset{margin-left:auto;margin-right:80px}\n"],
            kd: 2
        });
        var Ueb, Veb, Web, Xeb, Yeb, Zeb, $eb, afb, bfb, cfb, dfb, gfb, hfb, ifb, jfb, kfb, lfb;
        Ueb = function(a) {
            return _.Pg( (b, c) => {
                var d = !1
                  , e = 0;
                b.subscribe(new _.Qg(c,f => (d || (d = !a(f, e++))) && c.next(f)))
            }
            )
        }
        ;
        _.$U = function(a, b, c, d, e, f, g, h) {
            var k = _.t()
              , l = _.Df.Ff;
            f = _.uf(l.Na, f);
            _.Vj(k, l, a, b, c, d, e, f, void 0, g, h);
            return _.$U
        }
        ;
        Veb = function(a) {
            a & 1 && (_.Qf(),
            _.I(0, "svg", 2),
            _.L(1, "polygon", 3),
            _.K())
        }
        ;
        Web = function(a) {
            if (a & 1) {
                let b = _.t();
                _.lk(0, "div", 0);
                _.Gk("click", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(c.closed.emit("click"))
                })("animationstart", function(c) {
                    _.v(b);
                    var d = _.O();
                    return _.w(d.zAa(c.animationName))
                })("animationend", function(c) {
                    _.v(b);
                    var d = _.O();
                    return _.w(d.zS(c.animationName))
                })("animationcancel", function(c) {
                    _.v(b);
                    var d = _.O();
                    return _.w(d.zS(c.animationName))
                });
                _.lk(1, "div", 1);
                _.Ik(2);
                _.mk()()
            }
            a & 2 && (a = _.O(),
            _.cl(a.XK),
            _.Q("mat-menu-panel-animations-disabled", a.df)("mat-menu-panel-exit-animation", a.CS === "void")("mat-menu-panel-animating", a.hz()),
            _.kk("id", a.VI),
            _.bk("aria-label", a.ariaLabel || null)("aria-labelledby", a.Dl || null)("aria-describedby", a.LC || null))
        }
        ;
        Xeb = function(a) {
            a & 1 && _.L(0, "img", 4);
            a & 2 && (a = _.O(2),
            _.H("src", a.wk()))
        }
        ;
        Yeb = function(a) {
            a & 1 && (_.I(0, "span", 9)(1, "mat-icon", 11),
            _.R(2, "broken_image"),
            _.K()())
        }
        ;
        Zeb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "img", 12);
                _.M("error", function() {
                    _.v(b);
                    _.O(3).l5.set(!0);
                    return _.w()
                });
                _.K()
            }
            a & 2 && (a = _.O(3),
            _.H("src", a.vl()))
        }
        ;
        $eb = function(a) {
            a & 1 && _.E(0, Yeb, 3, 0, "span", 9)(1, Zeb, 1, 1, "img", 10);
            a & 2 && (a = _.O(2),
            _.F(a.YRa() ? 0 : 1))
        }
        ;
        afb = function(a) {
            a & 1 && (_.I(0, "mat-icon"),
            _.R(1),
            _.K());
            a & 2 && (a = _.O(2),
            _.Q("fill", a.cj()),
            _.B(),
            _.S(a.icon()))
        }
        ;
        bfb = function(a) {
            a & 1 && (_.I(0, "span", 8),
            _.R(1),
            _.K());
            a & 2 && (a = _.O(2),
            _.B(),
            _.il(" ", a.caption(), " "))
        }
        ;
        cfb = function(a) {
            a & 1 && (_.I(0, "span", 3),
            _.E(1, Xeb, 1, 1, "img", 4)(2, $eb, 2, 1)(3, afb, 2, 3, "mat-icon", 5),
            _.I(4, "span", 6)(5, "span", 7),
            _.R(6),
            _.K(),
            _.E(7, bfb, 2, 1, "span", 8),
            _.K()());
            a & 2 && (a = _.O(),
            _.B(),
            _.F(a.wk() ? 1 : a.vl() !== void 0 ? 2 : a.icon() ? 3 : -1),
            _.B(5),
            _.S(a.label()),
            _.B(),
            _.F(a.caption() ? 7 : -1))
        }
        ;
        dfb = function(a) {
            a & 1 && _.qk(0)
        }
        ;
        gfb = function(a, b) {
            if (a & 1) {
                let c = _.t();
                _.I(0, "a", 13);
                _.M("click", function(d) {
                    _.v(c);
                    var e = _.O();
                    return _.w(efb(e, d))
                })("auxclick", function(d) {
                    _.v(c);
                    var e = _.O();
                    return _.w(ffb(e, d))
                });
                _.Wj(1, dfb, 1, 0, "ng-container", 14);
                _.K()
            }
            if (a & 2) {
                a = _.O();
                let c = _.Xk(1);
                _.H("href", b.href, _.Di)("target", a.yM())("rel", a.lia())("disabled", a.disabled())("role", a.role());
                _.D("aria-checked", a.ariaChecked());
                _.H("matTooltip", a.zsa())("matTooltipDisabled", a.AW())("matTooltipPosition", "after");
                _.B();
                _.H("ngTemplateOutlet", c)
            }
        }
        ;
        hfb = function(a) {
            a & 1 && _.qk(0)
        }
        ;
        ifb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "button", 15);
                _.M("click", function(c) {
                    _.v(b);
                    var d = _.O();
                    return _.w(efb(d, c))
                })("auxclick", function(c) {
                    _.v(b);
                    var d = _.O();
                    return _.w(ffb(d, c))
                });
                _.Wj(1, hfb, 1, 0, "ng-container", 14);
                _.K()
            }
            if (a & 2) {
                a = _.O();
                let b = _.Xk(1);
                _.H("disabled", a.disabled())("role", a.role());
                _.D("aria-checked", a.ariaChecked());
                _.H("matMenuTriggerFor", a.iF())("matTooltip", a.zsa())("matTooltipDisabled", a.AW())("matTooltipPosition", "after");
                _.B();
                _.H("ngTemplateOutlet", b)
            }
        }
        ;
        jfb = function(a) {
            return a.button === 0 && !a.ctrlKey && !a.metaKey && !a.shiftKey && !a.altKey
        }
        ;
        kfb = function(a) {
            a.Db = !0;
            return a
        }
        ;
        lfb = class extends _.DOa {
            constructor() {
                super(...arguments);
                this.ob = "program"
            }
            na(a) {
                super.na(a);
                this.tk && this.tk.focus(this.ob)
            }
        }
        ;
        var aV = new _.df("MAT_MENU_PANEL");
        var mfb;
        mfb = [[["mat-icon"], ["", "matMenuItemIcon", ""]], "*"];
        _.bV = class {
            constructor() {
                this.Kb = _.r(_.vh);
                this.hc = _.r(_.Rs);
                this.ha = _.r(_.zG);
                this.ka = _.r(aV, {
                    optional: !0
                });
                this.Oc = _.r(_.gy);
                this.role = "menuitem";
                this.Mf = this.Ad = this.disabled = !1;
                this.ma = new _.Ej;
                this.Us = new _.Ej;
                this.Ys = this.nda = !1;
                _.r(_.$s).load(_.oH);
                var a, b;
                (a = this.ka) == null || (b = a.E1) == null || b.call(a, this);
                this.oa = this.Kb.Ja.tagName === "A"
            }
            focus(a, b) {
                this.ha && a ? _.yG(this.ha, this.Bu(), a, b) : this.Bu().focus(b);
                this.Us.next(this)
            }
            Pi() {
                this.ha && _.wG(this.ha, this.Kb, !1)
            }
            zc() {
                this.ha && _.xG(this.ha, this.Kb);
                this.ka && this.ka.removeItem && this.ka.removeItem(this);
                this.ma.complete();
                this.Us.complete()
            }
            uS() {
                return this.disabled && !this.Ad ? "-1" : "0"
            }
            u0() {
                return this.oa ? this.disabled || null : this.disabled && this.Ad ? !0 : null
            }
            dAa() {
                return this.Ad || !this.disabled ? null : !0
            }
            Bu() {
                return this.Kb.Ja
            }
            Nza(a) {
                this.disabled && (a.preventDefault(),
                a.stopPropagation())
            }
            mAa() {
                this.ma.next(this)
            }
            Dja() {
                var a = this.Kb.Ja.cloneNode(!0)
                  , b = a.querySelectorAll("mat-icon, .material-icons, .material-symbols-outlined, .material-symbols-rounded, .material-symbols-sharp");
                for (let d = 0; d < b.length; d++)
                    b[d].remove();
                var c;
                return ((c = a.textContent) == null ? void 0 : c.trim()) || ""
            }
            ua(a) {
                this.nda = a;
                _.Yx(this.Oc)
            }
            na(a) {
                this.Ys = a;
                _.Yx(this.Oc)
            }
            cL() {
                return this.hc && this.hc.activeElement === this.Bu()
            }
        }
        ;
        _.bV.va = function(a) {
            return new (a || _.bV)
        }
        ;
        _.bV.Ma = _.z({
            type: _.bV,
            Fa: [["", "mat-menu-item", ""]],
            Bc: [1, "mat-mdc-menu-item", "mat-focus-indicator"],
            nc: 12,
            Ob: function(a, b) {
                a & 1 && _.M("click", function(c) {
                    return b.Nza(c)
                })("mouseenter", function() {
                    return b.mAa()
                });
                a & 2 && (_.bk("role", b.role)("tabindex", b.uS())("aria-disabled", b.u0())("disabled", b.dAa()),
                _.Q("mat-mdc-menu-item-highlighted", b.nda)("mat-mdc-menu-item-submenu-trigger", b.Ys)("mat-mdc-menu-item-disabled", b.disabled)("mat-mdc-menu-item-disabled-interactive", b.Ad))
            },
            inputs: {
                role: "role",
                disabled: [2, "disabled", "disabled", _.Dl],
                Ad: [2, "disabledInteractive", "disabledInteractive", _.Dl],
                Mf: [2, "disableRipple", "disableRipple", _.Dl]
            },
            Rd: ["matMenuItem"],
            Md: ["mat-icon, [matMenuItemIcon]", "*"],
            Ka: 5,
            La: 3,
            Na: [[1, "mat-mdc-menu-item-text"], ["matRipple", "", 1, "mat-mdc-menu-ripple", 3, "matRippleDisabled", "matRippleTrigger"], ["viewBox", "0 0 5 10", "focusable", "false", "aria-hidden", "true", 1, "mat-mdc-menu-submenu-icon"], ["points", "0,0 5,5 0,10"]],
            template: function(a, b) {
                a & 1 && (_.Hk(mfb),
                _.Ik(0),
                _.I(1, "span", 0),
                _.Ik(2, 1),
                _.K(),
                _.L(3, "div", 1),
                _.E(4, Veb, 2, 0, "svg", 2));
                a & 2 && (_.B(3),
                _.H("matRippleDisabled", b.Mf || b.disabled)("matRippleTrigger", b.Bu()),
                _.B(),
                _.F(b.Ys ? 4 : -1))
            },
            dependencies: [_.xH],
            kd: 2
        });
        var nfb;
        nfb = new _.df("MatMenuContent");
        _.cV = class {
            constructor() {
                this.ma = _.r(_.Kk);
                this.Ts = _.r(_.vl);
                this.Vc = _.r(_.Nh);
                this.na = _.r(_.Lk);
                this.hc = _.r(_.Rs);
                this.Oc = _.r(_.gy);
                this.oa = new _.Ej
            }
            attach(a={}) {
                this.ha || (this.ha = new _.jH(this.ma,this.na));
                this.detach();
                this.ka || (this.ka = new _.Wpa(this.hc.createElement("div"),this.Ts,this.Vc));
                var b = this.ma.yd.Ja;
                b.parentNode.insertBefore(this.ka.ka, b);
                _.Yx(this.Oc);
                this.ha.attach(this.ka, a);
                this.oa.next()
            }
            detach() {
                var a;
                ((a = this.ha) == null ? 0 : a.ka) && this.ha.detach()
            }
            zc() {
                this.detach();
                var a;
                (a = this.ka) == null || a.dispose()
            }
        }
        ;
        _.cV.va = function(a) {
            return new (a || _.cV)
        }
        ;
        _.cV.nd = _.sg({
            type: _.cV,
            Fa: [["ng-template", "matMenuContent", ""]],
            features: [_.ml([{
                Ub: nfb,
                wf: _.cV
            }])]
        });
        var ofb, pfb, qfb;
        ofb = new _.df("mat-menu-default-options",{
            Hc: "root",
            Qa: () => ({
                CP: !1,
                yF: "after",
                CK: "below",
                Gd: "cdk-overlay-transparent-backdrop"
            })
        });
        pfb = function(a) {
            return a.ha.ha.Ra(_.Gj(a.ha), _.Hj(b => _.ah(...b.map(c => c.ma))))
        }
        ;
        qfb = function(a, b="program") {
            var c;
            (c = a.ua) == null || c.destroy();
            a.ua = _.Sh( () => {
                var d = a.wa();
                if (!d || !d.contains(document.activeElement)) {
                    let e = a.ka;
                    e.ob = b;
                    e.oa(0, 1);
                    !e.tk && d && d.focus()
                }
            }
            , {
                Mb: a.Vc
            })
        }
        ;
        _.dV = class {
            get yF() {
                return this.Aw
            }
            set yF(a) {
                this.Aw = a;
                this.uQ()
            }
            get CK() {
                return this.Ba
            }
            set CK(a) {
                this.Ba = a;
                this.uQ()
            }
            get Dd() {
                return this.na
            }
            set Dd(a) {
                var b = this.na
                  , c = Object.assign({}, this.XK);
                b && b.length && b.split(" ").forEach(d => {
                    c[d] = !1
                }
                );
                (this.na = a) && a.length && (a.split(" ").forEach(d => {
                    c[d] = !0
                }
                ),
                this.Kb.Ja.className = "");
                this.XK = c
            }
            get classList() {
                return this.Dd
            }
            set classList(a) {
                this.Dd = a
            }
            constructor() {
                this.Kb = _.r(_.vh);
                this.Oc = _.r(_.gy);
                this.Vc = _.r(_.Nh);
                this.df = _.qt();
                this.ha = new _.Nk;
                this.XK = {};
                this.CS = "void";
                this.oa = new _.Ej;
                this.hz = _.P(!1);
                this.CP = !1;
                this.na = "";
                this.close = this.closed = new _.pu;
                this.VI = _.r(_.bt).Sc("mat-menu-panel-");
                var a = _.r(ofb);
                this.k8 = a.k8 || "";
                this.Aw = a.yF;
                this.Ba = a.CK;
                this.Gd = a.Gd;
                this.CP = a.CP;
                this.nh = a.nh
            }
            ho() {
                this.uQ()
            }
            ns() {
                this.Ga();
                this.ka = _.COa(_.LG(kfb(new lfb(this.ha)))).FB(a => a.disabled && !a.Ad);
                this.ka.Sa.subscribe( () => this.closed.emit("tab"));
                this.ha.ha.Ra(_.Gj(this.ha), _.Hj(a => _.ah(...a.map(b => b.Us)))).subscribe(a => _.MG(this.ka, a));
                this.ha.ha.subscribe(a => {
                    var b = this.ka, c;
                    this.CS === "enter" && ((c = b.tk) == null ? 0 : c.cL()) && (a = a.toArray(),
                    c = Math.max(0, Math.min(a.length - 1, b.wa || 0)),
                    a[c] && !a[c].disabled ? b.na(c) : _.NG(b))
                }
                )
            }
            zc() {
                var a;
                (a = this.ka) == null || a.destroy();
                this.ha.destroy();
                this.closed.complete();
                var b;
                (b = this.ua) == null || b.destroy();
                clearTimeout(this.ma)
            }
            E1() {}
            removeItem() {}
            gz(a) {
                var b = a.keyCode
                  , c = this.ka;
                switch (b) {
                case 27:
                    _.ws(a) || (a.preventDefault(),
                    this.closed.emit("keydown"));
                    break;
                case 37:
                    this.DP && this.direction === "ltr" && this.closed.emit("keydown");
                    break;
                case 39:
                    this.DP && this.direction === "rtl" && this.closed.emit("keydown");
                    break;
                default:
                    if (b === 38 || b === 40)
                        c.ob = "keyboard";
                    c.Oa(a)
                }
            }
            uQ(a=this.yF, b=this.CK) {
                this.XK = Object.assign({}, this.XK, {
                    ["mat-menu-before"]: a === "before",
                    ["mat-menu-after"]: a === "after",
                    ["mat-menu-above"]: b === "above",
                    ["mat-menu-below"]: b === "below"
                });
                _.Yx(this.Oc)
            }
            zS(a) {
                var b = a === "_mat-menu-exit";
                if (b || a === "_mat-menu-enter")
                    b && (clearTimeout(this.ma),
                    this.ma = void 0),
                    this.oa.next(b ? "void" : "enter"),
                    this.hz.set(!1)
            }
            zAa(a) {
                a !== "_mat-menu-enter" && a !== "_mat-menu-exit" || this.hz.set(!0)
            }
            Aa(a) {
                this.CS = a ? "enter" : "void";
                if (a) {
                    if (this.ka.wa === 0) {
                        let b = this.wa();
                        b && (b.scrollTop = 0)
                    }
                } else
                    this.df || (this.ma = setTimeout( () => this.zS("_mat-menu-exit"), 200));
                this.df && setTimeout( () => {
                    this.zS(a ? "_mat-menu-enter" : "_mat-menu-exit")
                }
                );
                _.Yx(this.Oc)
            }
            Ga() {
                this.Oca.ha.Ra(_.Gj(this.Oca)).subscribe(a => {
                    this.ha.reset(a.filter(b => b.ka === this));
                    _.Qk(this.ha)
                }
                )
            }
            wa() {
                var a = null;
                this.ha.length && (a = this.ha.first.Bu().closest('[role="menu"]'));
                return a
            }
        }
        ;
        _.dV.va = function(a) {
            return new (a || _.dV)
        }
        ;
        _.dV.Ma = _.z({
            type: _.dV,
            Fa: [["mat-menu"]],
            Yg: function(a, b, c) {
                a & 1 && _.Ok(c, nfb, 5)(c, _.bV, 5)(c, _.bV, 4);
                if (a & 2) {
                    let d;
                    _.Rk(d = _.Sk()) && (b.K6 = d.first);
                    _.Rk(d = _.Sk()) && (b.Oca = d);
                    _.Rk(d = _.Sk()) && (b.items = d)
                }
            },
            qc: function(a, b) {
                a & 1 && _.Pk(_.Kk, 5);
                if (a & 2) {
                    let c;
                    _.Rk(c = _.Sk()) && (b.hu = c.first)
                }
            },
            nc: 3,
            Ob: function(a) {
                a & 2 && _.bk("aria-label", null)("aria-labelledby", null)("aria-describedby", null)
            },
            inputs: {
                Gd: "backdropClass",
                ariaLabel: [0, "aria-label", "ariaLabel"],
                Dl: [0, "aria-labelledby", "ariaLabelledby"],
                LC: [0, "aria-describedby", "ariaDescribedby"],
                yF: "xPosition",
                CK: "yPosition",
                CP: [2, "overlapTrigger", "overlapTrigger", _.Dl],
                nh: [2, "hasBackdrop", "hasBackdrop", a => a == null ? null : _.Dl(a)],
                Dd: [0, "class", "panelClass"],
                classList: "classList"
            },
            outputs: {
                closed: "closed",
                close: "close"
            },
            Rd: ["matMenu"],
            features: [_.ml([{
                Ub: aV,
                wf: _.dV
            }])],
            Md: ["*"],
            Ka: 1,
            La: 0,
            Na: [["tabindex", "-1", "role", "menu", 1, "mat-mdc-menu-panel", 3, "click", "animationstart", "animationend", "animationcancel", "id"], [1, "mat-mdc-menu-content"]],
            template: function(a) {
                a & 1 && (_.Hk(),
                _.$U(0, Web, 3, 12, "ng-template"))
            },
            styles: ['mat-menu{display:none}.mat-mdc-menu-content{margin:0;padding:8px 0;outline:0}.mat-mdc-menu-content,.mat-mdc-menu-content .mat-mdc-menu-item .mat-mdc-menu-item-text{-moz-osx-font-smoothing:grayscale;-webkit-font-smoothing:antialiased;flex:1;white-space:normal;font-family:var(--%NS%mat-menu-item-label-text-font, var(--%NS%mat-sys-label-large-font));line-height:var(--%NS%mat-menu-item-label-text-line-height, var(--%NS%mat-sys-label-large-line-height));font-size:var(--%NS%mat-menu-item-label-text-size, var(--%NS%mat-sys-label-large-size));letter-spacing:var(--%NS%mat-menu-item-label-text-tracking, var(--%NS%mat-sys-label-large-tracking));font-weight:var(--%NS%mat-menu-item-label-text-weight, var(--%NS%mat-sys-label-large-weight))}@keyframes _mat-menu-enter{from{opacity:0;transform:scale(0.8)}to{opacity:1;transform:none}}@keyframes _mat-menu-exit{from{opacity:1}to{opacity:0}}.mat-mdc-menu-panel{min-width:112px;max-width:280px;overflow:auto;box-sizing:border-box;outline:0;animation:_mat-menu-enter 120ms cubic-bezier(0, 0, 0.2, 1);border-radius:var(--%NS%mat-menu-container-shape, var(--%NS%mat-sys-corner-extra-small));background-color:var(--%NS%mat-menu-container-color, var(--%NS%mat-sys-surface-container));box-shadow:var(--%NS%mat-menu-container-elevation-shadow, 0px 3px 1px -2px rgba(0, 0, 0, 0.2), 0px 2px 2px 0px rgba(0, 0, 0, 0.14), 0px 1px 5px 0px rgba(0, 0, 0, 0.12));will-change:transform,opacity}.mat-mdc-menu-panel.mat-menu-panel-exit-animation{animation:_mat-menu-exit 100ms 25ms linear forwards}.mat-mdc-menu-panel.mat-menu-panel-animations-disabled{animation:none}.mat-mdc-menu-panel.mat-menu-panel-animating{pointer-events:none}.mat-mdc-menu-panel.mat-menu-panel-animating:has(.mat-mdc-menu-content:empty){display:none}@media(forced-colors: active){.mat-mdc-menu-panel{outline:solid 1px}}.mat-mdc-menu-panel .mat-divider{border-top-color:var(--%NS%mat-menu-divider-color, var(--%NS%mat-sys-surface-variant));margin-bottom:var(--%NS%mat-menu-divider-bottom-spacing, 8px);margin-top:var(--%NS%mat-menu-divider-top-spacing, 8px)}.mat-mdc-menu-item{display:flex;position:relative;align-items:center;justify-content:flex-start;overflow:hidden;padding:0;cursor:pointer;width:100%;text-align:left;box-sizing:border-box;color:inherit;font-size:inherit;background:none;text-decoration:none;margin:0;min-height:48px;padding-left:var(--%NS%mat-menu-item-leading-spacing, 12px);padding-right:var(--%NS%mat-menu-item-trailing-spacing, 12px);-webkit-user-select:none;user-select:none;cursor:pointer;outline:none;border:none;-webkit-tap-highlight-color:rgba(0,0,0,0)}.mat-mdc-menu-item::-moz-focus-inner{border:0}[dir=rtl] .mat-mdc-menu-item{padding-left:var(--%NS%mat-menu-item-trailing-spacing, 12px);padding-right:var(--%NS%mat-menu-item-leading-spacing, 12px)}.mat-mdc-menu-item:has(.material-icons,.material-symbols-outlined,.material-symbols-rounded,.material-symbols-sharp,mat-icon,[matButtonIcon]){padding-left:var(--%NS%mat-menu-item-with-icon-leading-spacing, 12px);padding-right:var(--%NS%mat-menu-item-with-icon-trailing-spacing, 12px)}[dir=rtl] .mat-mdc-menu-item:has(.material-icons,.material-symbols-outlined,.material-symbols-rounded,.material-symbols-sharp,mat-icon,[matButtonIcon]){padding-left:var(--%NS%mat-menu-item-with-icon-trailing-spacing, 12px);padding-right:var(--%NS%mat-menu-item-with-icon-leading-spacing, 12px)}.mat-mdc-menu-item,.mat-mdc-menu-item:visited,.mat-mdc-menu-item:link{color:var(--%NS%mat-menu-item-label-text-color, var(--%NS%mat-sys-on-surface))}.mat-mdc-menu-item .mat-icon-no-color,.mat-mdc-menu-item .mat-mdc-menu-submenu-icon{color:var(--%NS%mat-menu-item-icon-color, var(--%NS%mat-sys-on-surface-variant))}.mat-mdc-menu-item[disabled],.mat-mdc-menu-item.mat-mdc-menu-item-disabled{cursor:default;opacity:.38}.mat-mdc-menu-item[disabled]::after,.mat-mdc-menu-item.mat-mdc-menu-item-disabled::after{display:block;position:absolute;content:"";top:0;left:0;bottom:0;right:0}.mat-mdc-menu-item:focus{outline:0}.mat-mdc-menu-item .mat-icon{flex-shrink:0;margin-right:var(--%NS%mat-menu-item-spacing, 12px);height:var(--%NS%mat-menu-item-icon-size, 24px);width:var(--%NS%mat-menu-item-icon-size, 24px)}[dir=rtl] .mat-mdc-menu-item{text-align:right}[dir=rtl] .mat-mdc-menu-item .mat-icon{margin-right:0;margin-left:var(--%NS%mat-menu-item-spacing, 12px)}.mat-mdc-menu-item:not([disabled]):hover{background-color:var(--%NS%mat-menu-item-hover-state-layer-color, color-mix(in srgb, var(--%NS%mat-sys-on-surface) calc(var(--%NS%mat-sys-hover-state-layer-opacity) * 100%), transparent))}.mat-mdc-menu-item:not([disabled]).cdk-program-focused,.mat-mdc-menu-item:not([disabled]).cdk-keyboard-focused,.mat-mdc-menu-item:not([disabled]).mat-mdc-menu-item-highlighted{background-color:var(--%NS%mat-menu-item-focus-state-layer-color, color-mix(in srgb, var(--%NS%mat-sys-on-surface) calc(var(--%NS%mat-sys-focus-state-layer-opacity) * 100%), transparent))}@media(forced-colors: active){.mat-mdc-menu-item{margin-top:1px}}.mat-mdc-menu-submenu-icon{width:var(--%NS%mat-menu-item-icon-size, 24px);height:10px;fill:currentColor;padding-left:var(--%NS%mat-menu-item-spacing, 12px)}[dir=rtl] .mat-mdc-menu-submenu-icon{padding-right:var(--%NS%mat-menu-item-spacing, 12px);padding-left:0}[dir=rtl] .mat-mdc-menu-submenu-icon polygon{transform:scaleX(-1);transform-origin:center}@media(forced-colors: active){.mat-mdc-menu-submenu-icon{fill:CanvasText}}.mat-mdc-menu-item .mat-mdc-menu-ripple{top:0;left:0;right:0;bottom:0;position:absolute;pointer-events:none}\n'],
            kd: 2
        });
        var rfb = new _.df("mat-menu-scroll-strategy",{
            Hc: "root",
            Qa: () => {
                var a = _.r(_.Nh);
                return () => _.Vs(a)
            }
        })
          , eV = new WeakMap
          , tfb = function(a, b) {
            if (!a.Mc) {
                let c = sfb(a, b);
                a.Ug(b, c.dk);
                a.Mc = _.mt(a.Vc, c);
                a.Mc.Yx().subscribe(d => {
                    a.ha instanceof _.dV && a.ha.gz(d)
                }
                )
            }
            return a.Mc
        }
          , sfb = function(a, b) {
            return new _.et({
                dk: _.dH(_.XOa(_.cH(_.Ys(a.Vc, a.tc()))), ".mat-menu-panel, .mat-mdc-menu-panel"),
                Gd: b.Gd || "cdk-overlay-transparent-backdrop",
                Dd: b.k8,
                ek: a.ni(),
                direction: a.Aa || "ltr",
                hD: a.df
            })
        }
          , fV = class {
            get ha() {
                return this.Va
            }
            set ha(a) {
                if (a !== this.Va) {
                    this.Va = a;
                    this.Sa.unsubscribe();
                    a ? this.Sa = a.close.subscribe(c => {
                        this.ua(c);
                        c !== "click" && c !== "tab" || !this.ka || this.ka.closed.emit(c)
                    }
                    ) : this.ua();
                    var b;
                    (b = this.oa) == null || b.na(this.Ys())
                }
            }
            constructor(a) {
                this.fe = a;
                this.Uf = _.r(_.vh);
                this.Xd = _.r(_.Lk);
                this.oa = _.r(_.bV, {
                    optional: !0,
                    self: !0
                });
                this.Aa = _.r(_.ct, {
                    optional: !0
                });
                this.Db = _.r(_.zG);
                this.Rb = _.r(_.Yj);
                this.Vc = _.r(_.Nh);
                this.ni = _.r(rfb);
                this.Oc = _.r(_.gy);
                this.df = _.qt();
                this.Mc = null;
                this.na = !1;
                this.Sa = this.Oa = _.xg.ha;
                this.ma = void 0;
                this.Va = null;
                var b = _.r(aV, {
                    optional: !0
                });
                this.ka = b instanceof _.dV ? b : void 0
            }
            zc() {
                this.ha && this.mb(this.ha) && eV.delete(this.ha);
                var a;
                (a = this.Ab) == null || a.unsubscribe();
                this.Sa.unsubscribe();
                this.Oa.unsubscribe();
                this.Mc && (this.Mc.dispose(),
                this.Mc = null)
            }
            get PA() {
                return this.na
            }
            get dir() {
                return this.Aa && this.Aa.value === "rtl" ? "rtl" : "ltr"
            }
            Ys() {
                return !!(this.oa && this.ka && this.ha)
            }
            Ia() {
                var a;
                (a = this.ha) == null || a.close.emit()
            }
            xC(a) {
                if (!this.Vg()) {
                    var b = this.ha;
                    if (!this.na && b) {
                        var c;
                        (c = this.Ab) == null || c.unsubscribe();
                        c = eV.get(b);
                        eV.set(b, this);
                        c && c !== this && c.Ia();
                        c = tfb(this, b);
                        var d = c.Ec
                          , e = d.dk;
                        this.jg(b, e);
                        if (this.fe)
                            d.nh = b.nh == null ? !this.Ys() : b.nh;
                        else {
                            let f;
                            d.nh = (f = b.nh) != null ? f : !1
                        }
                        if (!c.Jl()) {
                            c.attach(this.ye(b));
                            let f;
                            (f = b.K6) == null || f.attach(this.Cna)
                        }
                        this.Oa = this.Te().subscribe( () => this.Ia());
                        b.DP = this.Ys() ? this.ka : void 0;
                        b.direction = this.dir;
                        a && qfb(b, this.ma || "program");
                        this.xd(!0);
                        b instanceof _.dV && (b.Aa(!0),
                        b.ha.ha.Ra(_.Ij(b.close)).subscribe( () => {
                            _.TOa(_.cH(e, !1));
                            _.cH(e, !0)
                        }
                        ))
                    }
                }
            }
            focus(a, b) {
                this.Db && a ? _.yG(this.Db, this.Uf, a, b) : this.Uf.Ja.focus(b)
            }
            ua(a) {
                var b = this.Mc
                  , c = this.ha;
                if (b && this.PA) {
                    this.Oa.unsubscribe();
                    var d;
                    (d = this.Ab) == null || d.unsubscribe();
                    if (c instanceof _.dV && this.mb(c))
                        this.Ab = c.oa.Ra(_.zj()).subscribe( () => {
                            b.detach();
                            if (!eV.has(c)) {
                                let e;
                                (e = c.K6) == null || e.detach()
                            }
                        }
                        ),
                        c.Aa(!1);
                    else {
                        b.detach();
                        let e;
                        c == null || (e = c.K6) == null || e.detach()
                    }
                    c && this.mb(c) && eV.delete(c);
                    !this.ql || a !== "keydown" && this.ma && this.Ys() || this.focus(this.ma);
                    this.ma = void 0;
                    this.xd(!1)
                }
            }
            xd(a) {
                a !== this.na && ((this.na = a) ? this.fP.emit() : this.dP.emit(),
                this.Ys() && this.oa.ua(a),
                _.Yx(this.Oc))
            }
            Ug(a, b) {
                a.uQ && b.ye.subscribe(c => {
                    this.Rb.run( () => {
                        a.uQ(c.ha.Id === "start" ? "after" : "before", c.ha.Ud === "top" ? "below" : "above")
                    }
                    )
                }
                )
            }
            jg(a, b) {
                var [c,d] = a.yF === "before" ? ["end", "start"] : ["start", "end"]
                  , e = c
                  , f = d
                  , [g,h] = a.CK === "above" ? ["bottom", "top"] : ["top", "bottom"]
                  , [k,l] = [g, h]
                  , q = k
                  , u = l
                  , [x,y] = [e, f]
                  , C = x
                  , G = y
                  , J = 0;
                this.Ys() ? (G = e = a.yF === "before" ? "start" : "end",
                f = C = e === "end" ? "start" : "end",
                this.ka && (this.ob == null && (this.ob = (a = this.ka.items.first) ? a.Bu().offsetTop : 0),
                J = g === "bottom" ? this.ob : -this.ob)) : a.CP || (q = g === "top" ? "bottom" : "top",
                u = h === "top" ? "bottom" : "top");
                _.bH(b, [{
                    qe: e,
                    re: q,
                    Id: C,
                    Ud: g,
                    offsetY: J
                }, {
                    qe: f,
                    re: q,
                    Id: G,
                    Ud: g,
                    offsetY: J
                }, {
                    qe: e,
                    re: u,
                    Id: C,
                    Ud: h,
                    offsetY: -J
                }, {
                    qe: f,
                    re: u,
                    Id: G,
                    Ud: h,
                    offsetY: -J
                }])
            }
            Te() {
                var a = this.Vb(this.Mc)
                  , b = this.Mc.na
                  , c = this.ka ? this.ka.closed : _.Kg()
                  , d = this.ka ? pfb(this.ka).Ra(_.bh(e => this.na && e !== this.oa)) : _.Kg();
                return _.ah(a, c, d, b)
            }
            ye(a) {
                this.Ba && this.Ba.hu === a.hu || (this.Ba = new _.jH(a.hu,this.Xd));
                return this.Ba
            }
            mb(a) {
                return eV.get(a) === this
            }
            Vg() {
                return _.Dl(this.Uf.Ja.getAttribute("aria-disabled"))
            }
        }
        ;
        fV.va = function() {
            throw Error("ca");
        }
        ;
        fV.nd = _.sg({
            type: fV
        });
        var ufb;
        ufb = function(a) {
            var b = a.Mc;
            b && (_.aH(b.Ec.dk, a.wa),
            b.Qq())
        }
        ;
        _.gV = class extends fV {
            get Rl() {
                return this.ha
            }
            set Rl(a) {
                this.ha = a
            }
            constructor() {
                super(!1);
                this.wa = {
                    x: 0,
                    y: 0,
                    Oka: 0,
                    Pka: 0,
                    Jka: 0,
                    Kka: 0
                };
                this.Ed = !1;
                this.hc = _.r(_.Rs);
                this.Nb = _.r(_.Qs);
                this.Yd = _.r(_.Us);
                this.ql = !0;
                this.disabled = !1;
                this.fP = new _.pu;
                this.dP = new _.pu
            }
            zc() {
                super.zc();
                var a;
                (a = this.Ga) == null || a.unsubscribe()
            }
            jAa(a) {
                this.disabled || (a.preventDefault(),
                this.PA ? (this.Fc(a.clientX, a.clientY),
                ufb(this)) : this.Tf(a))
            }
            ua(a) {
                super.ua(a);
                var b;
                (b = this.Ga) == null || b.unsubscribe()
            }
            tc() {
                return this.wa
            }
            Vb(a) {
                return a.AP().Ra(Ueb( (b, c) => {
                    if (b.type === "contextmenu")
                        return this.Yc(_.Fs(b));
                    if (b.type === "auxclick") {
                        if (c === 0)
                            return !0;
                        this.Lb != null || (this.Lb = _.Ds(this.Uf.Ja) || this.hc);
                        return this.Yc(this.Lb.elementFromPoint(b.clientX, b.clientY))
                    }
                    return this.Ed && c === 0 && b.ctrlKey
                }
                ))
            }
            Yc(a) {
                if (!a)
                    return !1;
                var b = this.Uf.Ja;
                if (a === b || b.contains(a))
                    return !0;
                var c;
                b = (c = this.Mc) == null ? void 0 : c.Vn;
                return b === a || !(b == null || !b.contains(a))
            }
            Tf(a) {
                a.button === 2 ? this.ma = "mouse" : this.ma = a.button === 0 ? "keyboard" : void 0;
                this.Fc(a.clientX, a.clientY);
                this.Ed = a.ctrlKey;
                super.xC(!0);
                var b;
                (b = this.Ga) == null || b.unsubscribe();
                this.Ga = this.Yd.scrolled(0).subscribe( () => {
                    var c = _.VG(this.Nb)
                      , d = this.wa;
                    d.y = d.Pka + (d.Kka - c.top);
                    d.x = d.Oka + (d.Jka - c.left);
                    ufb(this)
                }
                )
            }
            Fc(a, b) {
                var c = _.VG(this.Nb)
                  , d = this.wa;
                d.x = d.Oka = a;
                d.y = d.Pka = b;
                d.Jka = c.left;
                d.Kka = c.top
            }
        }
        ;
        _.gV.va = function(a) {
            return new (a || _.gV)
        }
        ;
        _.gV.nd = _.sg({
            type: _.gV,
            Fa: [["", "matContextMenuTriggerFor", ""]],
            Bc: [1, "mat-context-menu-trigger"],
            nc: 3,
            Ob: function(a, b) {
                a & 1 && _.M("contextmenu", function(c) {
                    return b.jAa(c)
                });
                a & 2 && (_.bk("aria-controls", b.PA ? b.Rl == null ? null : b.Rl.VI : null),
                _.Q("mat-context-menu-trigger-disabled", b.disabled))
            },
            inputs: {
                Rl: [0, "matContextMenuTriggerFor", "menu"],
                Cna: [0, "matContextMenuTriggerData", "menuData"],
                ql: [0, "matContextMenuTriggerRestoreFocus", "restoreFocus"],
                disabled: [2, "matContextMenuTriggerDisabled", "disabled", _.Dl]
            },
            outputs: {
                fP: "menuOpened",
                dP: "menuClosed"
            },
            Rd: ["matContextMenuTrigger"],
            features: [_.Tj]
        });
        _.hV = class extends fV {
            get bda() {
                return this.Rl
            }
            set bda(a) {
                this.Rl = a
            }
            get Rl() {
                return this.ha
            }
            set Rl(a) {
                this.ha = a
            }
            constructor() {
                super(!0);
                this.Ga = _.xg.ha;
                this.ql = !0;
                this.IOa = this.fP = new _.pu;
                this.HOa = this.dP = new _.pu;
                this.Lb = _.r(_.dt).listen(this.Uf.Ja, "touchstart", a => {
                    _.vs(a) || (this.ma = "touch")
                }
                , {
                    passive: !0
                })
            }
            wa() {
                return super.Ys()
            }
            gK() {
                if (this.PA)
                    var a = this.xr();
                else
                    this.xC(!0),
                    a = void 0;
                return a
            }
            xr() {
                this.Ia()
            }
            Qq() {
                var a;
                (a = this.Mc) == null || a.Qq()
            }
            ns() {
                this.Nb()
            }
            zc() {
                super.zc();
                this.Lb();
                this.Ga.unsubscribe()
            }
            tc() {
                return this.Uf
            }
            Vb(a) {
                return a.Cn()
            }
            oAa(a) {
                _.us(a) || (this.ma = a.button === 0 ? "mouse" : void 0,
                this.wa() && a.preventDefault())
            }
            gz(a) {
                a = a.keyCode;
                if (a === 13 || a === 32)
                    this.ma = "keyboard";
                this.wa() && (a === 39 && this.dir === "ltr" || a === 37 && this.dir === "rtl") && (this.ma = "keyboard",
                this.xC(!0))
            }
            v0(a) {
                this.wa() ? (a.stopPropagation(),
                this.xC(!0)) : this.gK()
            }
            Nb() {
                this.wa() && this.ka && (this.Ga = pfb(this.ka).subscribe(a => {
                    var b;
                    a !== this.oa || a.disabled || ((b = this.ka) == null ? void 0 : b.CS) === "void" || (this.ma = "mouse",
                    this.xC(!1))
                }
                ))
            }
        }
        ;
        _.hV.va = function(a) {
            return new (a || _.hV)
        }
        ;
        _.hV.nd = _.sg({
            type: _.hV,
            Fa: [["", "mat-menu-trigger-for", ""], ["", "matMenuTriggerFor", ""]],
            Bc: [1, "mat-mdc-menu-trigger"],
            nc: 3,
            Ob: function(a, b) {
                a & 1 && _.M("click", function(c) {
                    return b.v0(c)
                })("mousedown", function(c) {
                    return b.oAa(c)
                })("keydown", function(c) {
                    return b.gz(c)
                });
                a & 2 && _.bk("aria-haspopup", b.Rl ? "menu" : null)("aria-expanded", b.PA)("aria-controls", b.PA ? b.Rl == null ? null : b.Rl.VI : null)
            },
            inputs: {
                bda: [0, "mat-menu-trigger-for", "_deprecatedMatMenuTriggerFor"],
                Rl: [0, "matMenuTriggerFor", "menu"],
                Cna: [0, "matMenuTriggerData", "menuData"],
                ql: [0, "matMenuTriggerRestoreFocus", "restoreFocus"]
            },
            outputs: {
                fP: "menuOpened",
                IOa: "onMenuOpen",
                dP: "menuClosed",
                HOa: "onMenuClose"
            },
            Rd: ["matMenuTrigger"],
            features: [_.Tj]
        });
        var vfb, efb, ffb, xfb, wfb;
        vfb = [[["flow-menu-item-actions"]]];
        efb = function(a, b) {
            if (a.disabled())
                b.preventDefault(),
                b.stopPropagation();
            else {
                var c = a.link();
                c && a.U5() && a.yM() !== "_blank" && b instanceof MouseEvent && jfb(b) && (b.preventDefault(),
                a.tb.navigate([c.href]));
                a.iF() || a.FW.emit()
            }
        }
        ;
        ffb = function(a, b) {
            a.disabled() ? b.preventDefault() : b.button === 1 && !a.iF() && a.FW.emit()
        }
        ;
        xfb = function(a) {
            var b = a.ha();
            if (!a.disabled() && b != null) {
                var c = a.RW();
                wfb(b);
                var d = _.iV.ha.get(b);
                d && d !== c && (d.xr(),
                _.iV.ha.delete(b));
                c && a.iF() && !c.PA && _.iV.ka.set(b, setTimeout( () => {
                    _.iV.ka.delete(b);
                    c.PA || (c.xC(!0),
                    _.iV.ha.set(b, c))
                }
                , 0))
            }
        }
        ;
        wfb = function(a) {
            var b = _.iV.ka.get(a);
            b != null && (clearTimeout(b),
            _.iV.ka.delete(a))
        }
        ;
        _.iV = class {
            constructor() {
                this.label = _.pl.required();
                this.disabled = _.T(!1);
                this.selected = _.T(!1);
                this.role = _.T("menuitem");
                this.ariaChecked = _.T();
                this.Xma = _.T(!1);
                this.caption = _.T();
                this.wl = _.T();
                this.icon = _.T();
                this.wk = _.T();
                this.cj = _.T(!1);
                this.vl = _.T();
                this.link = _.T();
                this.iF = _.T(null);
                this.DP = _.T(null);
                this.FW = _.V();
                this.yM = _.X( () => {
                    var a = this.link();
                    return a ? a.target !== void 0 ? a.target : this.U5() ? null : "_blank" : null
                }
                );
                this.lia = _.X( () => {
                    var a = this.link();
                    return a ? a.rel !== void 0 ? a.rel : this.yM() === "_blank" ? "noopener noreferrer" : null : null
                }
                );
                this.zsa = _.X( () => {
                    var a;
                    return (a = this.wl()) != null ? a : ""
                }
                );
                this.AW = _.X( () => !this.wl());
                this.U5 = _.X( () => {
                    var a = this.link();
                    return a && a.href ? a.href.startsWith("/") || !/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(a.href) : !1
                }
                );
                this.tb = _.r(_.TD);
                this.ka = _.r(aV, {
                    optional: !0
                });
                this.ha = _.X( () => {
                    var a;
                    return (a = this.ka) != null ? a : this.DP()
                }
                );
                this.pna = _.ql(_.bV);
                this.RW = _.ql(_.hV);
                this.l5 = _.P(!1);
                this.YRa = _.X( () => {
                    var a = this.vl();
                    return a === "" || a instanceof Error ? !0 : a === void 0 ? !1 : this.l5()
                }
                );
                _.r(_.Rh).mc( () => {
                    var a = this.ha();
                    a != null && (wfb(a),
                    this.RW() === _.iV.ha.get(a) && _.iV.ha.delete(a))
                }
                );
                _.sj( () => {
                    this.icon() && this.vl()
                }
                );
                _.sj( () => {
                    this.vl();
                    _.le( () => {
                        this.l5.set(!1)
                    }
                    )
                }
                );
                _.sj( () => {
                    var a = this.RW()
                      , b = this.pna()
                      , c = this.ha()
                      , d = this.iF();
                    a && c && (a.ka = c,
                    b && b.na(!!d))
                }
                )
            }
        }
        ;
        _.iV.ha = new WeakMap;
        _.iV.ka = new WeakMap;
        _.iV.va = function(a) {
            return new (a || _.iV)
        }
        ;
        _.iV.Ma = _.z({
            type: _.iV,
            Fa: [["flow-menu-item"]],
            qc: function(a, b) {
                a & 1 && _.Vk(b.pna, _.bV, 5)(b.RW, _.hV, 5);
                a & 2 && _.Wk(2)
            },
            nc: 2,
            Ob: function(a, b) {
                a & 1 && _.M("mouseenter", function() {
                    return xfb(b)
                })("click", function(c) {
                    b.Xma() && c.stopPropagation();
                    if (c.target === c.currentTarget)
                        if (b.disabled())
                            c.stopPropagation();
                        else if (b.iF())
                            c.stopPropagation(),
                            xfb(b);
                        else {
                            var d = b.link();
                            if (d && jfb(c))
                                if (b.U5() && b.yM() !== "_blank")
                                    c.preventDefault(),
                                    b.tb.navigate([d.href]);
                                else if (c = _.Yl(d.href),
                                c != null) {
                                    let e, f;
                                    _.jm(window, c, (e = b.yM()) != null ? e : "_self", (f = b.lia()) != null ? f : void 0)
                                }
                            b.FW.emit()
                        }
                });
                a & 2 && _.Q("selected", b.selected())
            },
            inputs: {
                label: [1, "label"],
                disabled: [1, "disabled"],
                selected: [1, "selected"],
                role: [1, "role"],
                ariaChecked: [1, "ariaChecked"],
                Xma: [1, "keepOpen"],
                caption: [1, "caption"],
                wl: [1, "tooltip"],
                icon: [1, "icon"],
                wk: [1, "iconUrl"],
                cj: [1, "isIconFilled"],
                vl: [1, "thumbnail"],
                link: [1, "link"],
                iF: [1, "submenu"],
                DP: [1, "parentMenu"]
            },
            outputs: {
                FW: "itemClick"
            },
            Md: ["flow-menu-item-actions"],
            Ka: 5,
            La: 1,
            Na: [["menuItemContent", ""], ["mat-menu-item", "", 1, "flow-internal-menu-item-link", 3, "href", "target", "rel", "disabled", "role", "aria-checked", "matTooltip", "matTooltipDisabled", "matTooltipPosition"], ["mat-menu-item", "", 1, "flow-internal-menu-item", 3, "disabled", "role", "aria-checked", "matMenuTriggerFor", "matTooltip", "matTooltipDisabled", "matTooltipPosition"], [1, "left-content"], ["aria-hidden", "true", 1, "icon-image", 3, "src"], [3, "fill"], [1, "item-text"], [1, "label"], [1, "caption"], ["aria-hidden", "true", 1, "broken-thumbnail-container"], ["aria-hidden", "true", 1, "thumbnail", 3, "src"], [1, "flow-icon-s"], ["aria-hidden", "true", 1, "thumbnail", 3, "error", "src"], ["mat-menu-item", "", 1, "flow-internal-menu-item-link", 3, "click", "auxclick", "href", "target", "rel", "disabled", "role", "aria-checked", "matTooltip", "matTooltipDisabled", "matTooltipPosition"], [4, "ngTemplateOutlet"], ["mat-menu-item", "", 1, "flow-internal-menu-item", 3, "click", "auxclick", "disabled", "role", "aria-checked", "matMenuTriggerFor", "matTooltip", "matTooltipDisabled", "matTooltipPosition"]],
            template: function(a, b) {
                a & 1 && (_.Hk(vfb),
                _.Wj(0, cfb, 8, 3, "ng-template", null, 0, _.nl),
                _.E(2, gfb, 2, 10, "a", 1)(3, ifb, 2, 8, "button", 2),
                _.Ik(4));
                if (a & 2) {
                    let c;
                    _.B(2);
                    _.F((c = b.link()) ? 2 : 3, c)
                }
            },
            dependencies: [_.jI, _.bV, _.hV, _.hJ, _.BA],
            styles: ['[_ngcontent-%COMP%]:root{--%NS%glass-raised-box-shadow:0 16px 32px -8px rgba(0,0,0,.4);--%NS%flow-panel-border-radius:20px}.blurred-backdrop[_ngcontent-%COMP%]{background-color:rgba(0,0,0,.32);backdrop-filter:blur(8px);-webkit-backdrop-filter:blur(8px)}mat-icon[_ngcontent-%COMP%]{font-variation-settings:"FILL" var(--%NS%flow-icon-fill,0),"wght" var(--%NS%flow-icon-wght,400)}mat-icon.flow-icon-s[_ngcontent-%COMP%]{font-size:1rem;height:1rem;width:1rem;line-height:1rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-m[_ngcontent-%COMP%]{font-size:1.125rem;height:1.125rem;width:1.125rem;line-height:1.125rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-l[_ngcontent-%COMP%]{font-size:1.25rem;height:1.25rem;width:1.25rem;line-height:1.25rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-xl[_ngcontent-%COMP%]{font-size:1.5rem;height:1.5rem;width:1.5rem;line-height:1.5rem;--%NS%flow-icon-wght:300}mat-icon.fill[_ngcontent-%COMP%]{--%NS%flow-icon-fill:1}@-webkit-keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@-webkit-keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@-webkit-keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}@keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}[_nghost-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-orient:horizontal;-webkit-box-direction:normal;-webkit-flex-direction:row;-moz-box-orient:horizontal;-moz-box-direction:normal;-ms-flex-direction:row;flex-direction:row;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.75rem;border-radius:16px;min-height:2.125rem;-webkit-transition:background-color .1s ease-in-out;transition:background-color .1s ease-in-out;-webkit-user-select:none;-moz-user-select:none;-ms-user-select:none;user-select:none;padding:.5rem 1rem}[_nghost-%COMP%]:has(.left-content>.icon-image:first-child), [_nghost-%COMP%]:has(.left-content>mat-icon:first-child){-webkit-padding-start:.75rem;-moz-padding-start:.75rem;padding-inline-start:.75rem}[_nghost-%COMP%]:has(img.thumbnail,.broken-thumbnail-container){padding-top:.25rem;padding-bottom:.25rem;-webkit-padding-start:.25rem;-moz-padding-start:.25rem;padding-inline-start:.25rem}[_nghost-%COMP%]:has(flow-menu-item-actions){padding-top:.25rem;padding-bottom:.25rem;-webkit-padding-end:.5rem;-moz-padding-end:.5rem;padding-inline-end:.5rem}[_nghost-%COMP%]:not(:has(.flow-internal-menu-item[disabled],.flow-internal-menu-item[aria-disabled=true])):hover{background-color:var(--%NS%foreground-state-10);cursor:pointer}[_nghost-%COMP%]:not(:has(.flow-internal-menu-item[disabled],.flow-internal-menu-item[aria-disabled=true])):active{background-color:var(--%NS%foreground-state-15)}.selected[_nghost-%COMP%]{background-color:var(--%NS%foreground-state-15)}.flow-internal-menu-item[_ngcontent-%COMP%], .flow-internal-menu-item-link[_ngcontent-%COMP%]{border:none;background:transparent;-webkit-box-flex:1;-webkit-flex:1;-moz-box-flex:1;-ms-flex:1;flex:1;min-width:0;padding:0;min-height:0}.flow-internal-menu-item-link[_ngcontent-%COMP%]:not([disabled],[aria-disabled=true]):active, .flow-internal-menu-item[_ngcontent-%COMP%]:not([disabled],[aria-disabled=true]):active{box-shadow:0 4px 32px -8px rgba(0,0,0,.2);-webkit-filter:blur(1.5px);filter:blur(1.5px);backdrop-filter:blur(25px);-webkit-backdrop-filter:blur(25px)}.flow-internal-menu-item-link[_ngcontent-%COMP%]:disabled, .flow-internal-menu-item[_ngcontent-%COMP%]:disabled, .flow-internal-menu-item[aria-disabled=true][_ngcontent-%COMP%], [aria-disabled=true].flow-internal-menu-item-link[_ngcontent-%COMP%]{color:var(--%NS%foreground-state-50);cursor:default;pointer-events:auto}.flow-internal-menu-item-link[_ngcontent-%COMP%]:disabled   .caption[_ngcontent-%COMP%], .flow-internal-menu-item-link[_ngcontent-%COMP%]:disabled   mat-icon[_ngcontent-%COMP%], .flow-internal-menu-item[_ngcontent-%COMP%]:disabled   .caption[_ngcontent-%COMP%], .flow-internal-menu-item[_ngcontent-%COMP%]:disabled   mat-icon[_ngcontent-%COMP%], .flow-internal-menu-item[aria-disabled=true][_ngcontent-%COMP%]   .caption[_ngcontent-%COMP%], .flow-internal-menu-item[aria-disabled=true][_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%], [aria-disabled=true].flow-internal-menu-item-link[_ngcontent-%COMP%]   .caption[_ngcontent-%COMP%], [aria-disabled=true].flow-internal-menu-item-link[_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%]{color:var(--%NS%foreground-state-50)}.flow-internal-menu-item-link[_ngcontent-%COMP%]:disabled   img[_ngcontent-%COMP%], .flow-internal-menu-item[_ngcontent-%COMP%]:disabled   img[_ngcontent-%COMP%], .flow-internal-menu-item[aria-disabled=true][_ngcontent-%COMP%]   img[_ngcontent-%COMP%], [aria-disabled=true].flow-internal-menu-item-link[_ngcontent-%COMP%]   img[_ngcontent-%COMP%]{background:#d3d3d3 50%/cover no-repeat;background-blend-mode:color,normal;-webkit-filter:grayscale(100%);filter:grayscale(100%)}.flow-internal-menu-item-link[_ngcontent-%COMP%]{text-decoration:none;color:inherit}.left-content[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-orient:horizontal;-webkit-box-direction:normal;-webkit-flex-direction:row;-moz-box-orient:horizontal;-moz-box-direction:normal;-ms-flex-direction:row;flex-direction:row;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.75rem}.flow-internal-menu-item[_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%], .flow-internal-menu-item-link[_ngcontent-%COMP%]   mat-icon[_ngcontent-%COMP%]{margin-right:0;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0;--%NS%flow-icon-wght:300}.icon-image[_ngcontent-%COMP%]{width:1.5rem;height:1.5rem;border-radius:.1875rem;object-fit:cover;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}.thumbnail[_ngcontent-%COMP%]{width:2.5rem;height:2.5rem;border-radius:12px;object-fit:cover}.broken-thumbnail-container[_ngcontent-%COMP%]{width:2.5rem;height:2.5rem;border-radius:12px;display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;-webkit-box-pack:center;-webkit-justify-content:center;-moz-box-pack:center;-ms-flex-pack:center;justify-content:center;background-color:var(--%NS%foreground-state-15);-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}.item-text[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-orient:vertical;-webkit-box-direction:normal;-webkit-flex-direction:column;-moz-box-orient:vertical;-moz-box-direction:normal;-ms-flex-direction:column;flex-direction:column;-webkit-box-align:start;-webkit-align-items:flex-start;-moz-box-align:start;-ms-flex-align:start;align-items:flex-start;min-width:0}.label[_ngcontent-%COMP%]{display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.caption[_ngcontent-%COMP%]{font:var(--%NS%mat-sys-label-small);color:var(--%NS%foreground-state-75);display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}[_nghost-%COMP%]     .flow-internal-menu-item, [_nghost-%COMP%]     .flow-internal-menu-item-link{--%NS%mat-menu-item-hover-state-layer-color:transparent;--%NS%mat-menu-item-focus-state-layer-color:transparent}[_nghost-%COMP%]     .flow-internal-menu-item .mat-mdc-menu-item-text, [_nghost-%COMP%]     .flow-internal-menu-item-link .mat-mdc-menu-item-text{overflow:hidden}[_nghost-%COMP%]     .flow-internal-menu-item [matripple], [_nghost-%COMP%]     .flow-internal-menu-item-link [matripple]{display:none}[_nghost-%COMP%]     .flow-menu-item-actions{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-orient:horizontal;-webkit-box-direction:normal;-webkit-flex-direction:row;-moz-box-orient:horizontal;-moz-box-direction:normal;-ms-flex-direction:row;flex-direction:row;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;pointer-events:none}[_nghost-%COMP%]     .flow-menu-item-actions>*{pointer-events:auto}[_nghost-%COMP%]     .flow-menu-item-actions [flow-button], [_nghost-%COMP%]     .flow-menu-item-actions [flow-icon-button]{-webkit-backdrop-filter:none;backdrop-filter:none}\n/*# sourceMappingURL=menu_item.css.map */']
        });
        _.jV = function(a) {
            return a.ownerDocument
        }
        ;
        _.kV = function(a) {
            a = Math.max(0, a);
            var b = Math.floor(a % 60)
              , c = (d, e) => (new Intl.NumberFormat(void 0,{
                minimumIntegerDigits: e
            })).format(d);
            return `${c(Math.floor(a / 60), 1)}:${c(b, 2)}`
        }
        ;
        _.lV = class {
        }
        ;
        _.lV.va = function(a) {
            return new (a || _.lV)
        }
        ;
        _.lV.nd = _.sg({
            type: _.lV,
            Fa: [["flow-menu-item-actions"]],
            Bc: [1, "flow-menu-item-actions"]
        });
        var Trb, Urb, Wrb;
        Trb = function(a, b, c) {
            return _.A(function*() {
                var d = yield a.ha
                  , e = d[b];
                if (!e)
                    throw Error("Dh`" + b);
                return e.apply(d, c)
            })
        }
        ;
        Urb = class {
            constructor(a) {
                this.ha = a;
                a.then((0,
                _.Yp)(function() {}, this), () => {}
                , this)
            }
        }
        ;
        _.Xrb = function(a, b, c) {
            var d = Array(arguments.length - 2);
            for (var e = 2; e < arguments.length; e++)
                d[e - 2] = arguments[e];
            e = _.Vrb(a, b).then(f => f.apply(null, d), f => {
                f = Error("Eh`" + b + "`" + a + "`" + f, {
                    cause: f
                });
                delete Wrb[b];
                return _.oA(f)
            }
            );
            return new Urb(e)
        }
        ;
        Wrb = {};
        _.Vrb = function(a, b) {
            var c = Wrb[b];
            if (c)
                return c;
            c = (c = _.Zv(b)) ? _.nA(c) : (new _.lA(function(d, e) {
                var f = (new _.YA(document)).createElement("SCRIPT");
                f.async = !0;
                _.om(f, a);
                f.onload = f.onreadystatechange = function() {
                    f.readyState && f.readyState != "loaded" && f.readyState != "complete" || d()
                }
                ;
                f.onerror = g => {
                    e(Error("Fh`" + b + "`" + a, {
                        cause: g
                    }))
                }
                ;
                (document.head || document.getElementsByTagName("head")[0]).appendChild(f)
            }
            )).then(function() {
                var d = _.Zv(b);
                if (!d)
                    throw Error("Gh`" + b + "`" + a);
                return d
            });
            return Wrb[b] = c
        }
        ;
        (0,
        _.un)`https://www.google.com/tools/feedback/open_to_help_guide_lazy.js`;
        var Yrb = (0,
        _.un)`https://www.google.com/tools/feedback/help_api.js`
          , Zrb = class {
            constructor(a) {
                this.SO = a
            }
            ha(a) {
                var b = this
                  , c = arguments;
                return _.A(function*() {
                    var d = b.SO, e;
                    try {
                        c[0].commonConfig.frdProductData && (c[0].commonConfig.frdProductDataSerializedJspb = c[0].commonConfig.frdProductData.we(),
                        c[0].commonConfig.frdProductData = void 0)
                    } catch (f) {}
                    try {
                        if ((e = c[0].commonGseConfig) == null ? 0 : e.gseSessionOptions)
                            c[0].commonGseConfig.gseSessionOptionsSerializedJspb = c[0].commonGseConfig.gseSessionOptions.we(),
                            c[0].commonGseConfig.gseSessionOptions = void 0
                    } catch (f) {}
                    try {
                        c[0].startingFlow && (c[0].startingFlowSerializedJspb = c[0].startingFlow.we(),
                        c[0].startingFlow = void 0)
                    } catch (f) {}
                    e = c;
                    yield Trb(d, "openHelp", e)
                })
            }
            close() {
                var a = this
                  , b = arguments;
                return _.A(function*() {
                    yield Trb(a.SO, "close", b)
                })
            }
        }
        ;
        _.wZ = class {
            constructor() {
                this.locale = _.r(_.pj);
                this.Da = _.r(_.NH)
            }
            zP() {
                var a = this;
                return _.A(function*() {
                    if (a.Da.Ht())
                        try {
                            if (!a.ha) {
                                let b = _.Xrb(Yrb, "help.api.Lazy.create", "5266562");
                                a.ha = new Zrb(b)
                            }
                            yield a.ha.ha({
                                commonConfig: {
                                    authuser: 0,
                                    helpCenterPath: "flow",
                                    locale: a.locale
                                },
                                helpPanelConfig: {
                                    helpPanelTheme: 1
                                },
                                enableSendFeedback: !0
                            })
                        } catch (b) {
                            _.At(b, "warning")
                        }
                })
            }
        }
        ;
        _.wZ.va = function(a) {
            return new (a || _.wZ)
        }
        ;
        _.wZ.kb = _.mg({
            token: _.wZ,
            Qa: _.wZ.va
        });
        var yZ;
        _.xZ = function(a) {
            return a.ownerDocument.defaultView
        }
        ;
        yZ = class {
        }
        ;
        yZ.va = function(a) {
            return new (a || yZ)
        }
        ;
        yZ.kb = _.mg({
            token: yZ,
            Qa: yZ.va
        });
        _.zZ = class {
            constructor() {
                this.feedback = _.r(yZ);
                this.config = _.r(_.DKa)
            }
            jo(a, b) {
                try {
                    let c = Object.assign({}, this.config.productData);
                    if (b)
                        for (let[e,f] of Object.entries(b)) {
                            b = e;
                            let g = f;
                            g !== void 0 && g !== "" && (c[b] = g)
                        }
                    let d = Object.assign({}, this.config, a ? {
                        bucket: a
                    } : {}, {
                        productData: c
                    });
                    _.tUa(d, c, d.window)
                } catch (c) {
                    _.At(c, "warning")
                }
            }
        }
        ;
        _.zZ.va = function(a) {
            return new (a || _.zZ)
        }
        ;
        _.zZ.kb = _.mg({
            token: _.zZ,
            Qa: _.zZ.va
        });
        _.$rb = _.on((0,
        _.ln)`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 127 96">
  <path d="M81.15 0c-1.24 2.2-2.35 4.47-3.36 6.79-9.6-1.44-19.37-1.44-29 0-.98-2.32-2.12-4.6-3.36-6.79-9.02 1.54-17.8 4.24-26.14 8.06C2.78 32.53-1.69 56.37.53 79.89c9.67 7.15 20.51 12.6 32.05 16.09 2.6-3.49 4.9-7.2 6.87-11.06-3.74-1.39-7.35-3.13-10.81-5.15.91-.66 1.79-1.34 2.65-2 20.28 9.55 43.77 9.55 64.08 0 .86.71 1.74 1.39 2.65 2-3.46 2.04-7.07 3.76-10.84 5.18 1.97 3.86 4.27 7.58 6.87 11.06 11.54-3.49 22.38-8.92 32.05-16.06 2.63-27.28-4.5-50.92-18.82-71.86C98.98 4.27 90.19 1.57 81.18.05L81.15 0ZM42.28 65.41c-6.24 0-11.42-5.66-11.42-12.65s4.98-12.68 11.39-12.68 11.52 5.71 11.42 12.68c-.1 6.97-5.03 12.65-11.39 12.65Zm42.08 0c-6.26 0-11.39-5.66-11.39-12.65s4.98-12.68 11.39-12.68 11.49 5.71 11.39 12.68c-.1 6.97-5.03 12.65-11.39 12.65Z" fill="currentColor"/>
  </svg>`);
        _.asb = _.on((0,
        _.ln)`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none">
  <path d="M7.8 2H16.2C19.4 2 22 4.6 22 7.8V16.2C22 17.7383 21.3889 19.2135 20.3012 20.3012C19.2135 21.3889 17.7383 22 16.2 22H7.8C4.6 22 2 19.4 2 16.2V7.8C2 6.26174 2.61107 4.78649 3.69878 3.69878C4.78649 2.61107 6.26174 2 7.8 2ZM7.6 4C6.64522 4 5.72955 4.37928 5.05442 5.05442C4.37928 5.72955 4 6.64522 4 7.6V16.4C4 18.39 5.61 20 7.6 20H16.4C17.3548 20 18.2705 19.6207 18.9456 18.9456C19.6207 18.2705 20 17.3548 20 16.4V7.6C20 5.61 18.39 4 16.4 4H7.6ZM17.25 5.5C17.5815 5.5 17.8995 5.6317 18.1339 5.86612C18.3683 6.10054 18.5 6.41848 18.5 6.75C18.5 7.08152 18.3683 7.39946 18.1339 7.63388C17.8995 7.8683 17.5815 8 17.25 8C16.9185 8 16.6005 7.8683 16.3661 7.63388C16.1317 7.39946 16 7.08152 16 6.75C16 6.41848 16.1317 6.10054 16.3661 5.86612C16.6005 5.6317 16.9185 5.5 17.25 5.5ZM12 7C13.3261 7 14.5979 7.52678 15.5355 8.46447C16.4732 9.40215 17 10.6739 17 12C17 13.3261 16.4732 14.5979 15.5355 15.5355C14.5979 16.4732 13.3261 17 12 17C10.6739 17 9.40215 16.4732 8.46447 15.5355C7.52678 14.5979 7 13.3261 7 12C7 10.6739 7.52678 9.40215 8.46447 8.46447C9.40215 7.52678 10.6739 7 12 7ZM12 9C11.2044 9 10.4413 9.31607 9.87868 9.87868C9.31607 10.4413 9 11.2044 9 12C9 12.7956 9.31607 13.5587 9.87868 14.1213C10.4413 14.6839 11.2044 15 12 15C12.7956 15 13.5587 14.6839 14.1213 14.1213C14.6839 13.5587 15 12.7956 15 12C15 11.2044 14.6839 10.4413 14.1213 9.87868C13.5587 9.31607 12.7956 9 12 9Z" fill="currentColor"/>
  </svg>`);
        _.bsb = _.on((0,
        _.ln)`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 1227" fill="none">
  <path d="M714.163 519.284L1160.89 0H1055.03L667.137 450.887L357.328 0H0L468.492 681.821L0 1226.37H105.866L515.491 750.218L842.672 1226.37H1200L714.137 519.284H714.163ZM569.165 687.828L521.697 619.934L144.011 79.6944H306.615L611.412 515.685L658.88 583.579L1055.08 1150.3H892.476L569.165 687.854V687.828Z" fill="currentColor"/>
  </svg>`);
        var csb = function(a, b) {
            a & 1 && (_.lk(0, "span", 0),
            _.R(1),
            _.mk());
            a & 2 && (a = _.O(),
            _.bk("aria-label", a.fTa()),
            _.B(),
            _.S(b))
        }
          , dsb = function(a) {
            if (a !== void 0)
                switch (a) {
                case 1:
                    return "PLUS";
                case 2:
                case 5:
                case 6:
                    return "PRO";
                case 3:
                case 9:
                case 7:
                    return "ULTRA"
                }
        }
          , AZ = class {
            constructor() {
                this.pO = _.T(!1);
                this.ha = _.r(_.PJ);
                this.Da = _.r(_.NH);
                this.RZ = _.X( () => {
                    var a, b;
                    return (b = (a = this.pO() ? this.Da.ha.Db() : void 0) != null ? a : this.Da.BC()) != null ? b : dsb(this.ha.ha())
                }
                );
                this.fTa = _.X( () => {
                    var a = this.RZ();
                    return a ? `${a} tier` : void 0
                }
                )
            }
        }
        ;
        AZ.va = function(a) {
            return new (a || AZ)
        }
        ;
        AZ.Ma = _.z({
            type: AZ,
            Fa: [["flow-user-tier-chip"]],
            inputs: {
                pO: [1, "isPreviewLabelShown"]
            },
            Ka: 1,
            La: 1,
            Na: [[1, "tier-chip"]],
            template: function(a, b) {
                a & 1 && _.E(0, csb, 2, 2, "span", 0);
                if (a & 2) {
                    let c;
                    _.F((c = b.RZ()) ? 0 : -1, c)
                }
            },
            styles: ["[_nghost-%COMP%]{display:var(--%NS%flow-user-tier-chip-display,inline-flex)}.tier-chip[_ngcontent-%COMP%]{display:-webkit-inline-box;display:-webkit-inline-flex;display:-moz-inline-box;display:-ms-inline-flexbox;display:inline-flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;padding:.25rem .5rem;border-radius:8px;background:var(--%NS%foreground-state-15);font:var(--%NS%mat-sys-label-medium);text-transform:uppercase}"]
        });
        var esb = function(a) {
            a.ka || (a.ka = a.document.querySelector(".boqOnegoogleliteOgbOneGoogleBar"));
            return a.ka
        }
          , BZ = class {
            constructor() {
                this.document = _.r(_.Rs);
                this.observer = this.ha = this.ka = null
            }
            attach(a) {
                this.ha = a;
                var b = esb(this);
                if (b) {
                    a.appendChild(b);
                    let c;
                    (c = this.observer) == null || c.disconnect();
                    this.observer = null
                } else
                    this.observer || (this.observer = new MutationObserver( () => {
                        var c = esb(this);
                        if (c) {
                            this.ha && this.ha.appendChild(c);
                            var d;
                            (d = this.observer) == null || d.disconnect();
                            this.observer = null
                        }
                    }
                    ),
                    this.observer.observe(this.document.body, {
                        childList: !0,
                        subtree: !0
                    }))
            }
            detach(a) {
                if (this.ha === a) {
                    this.ha = null;
                    var b = esb(this);
                    b && b.parentElement === a && this.document.body.appendChild(b)
                }
            }
        }
        ;
        BZ.va = function(a) {
            return new (a || BZ)
        }
        ;
        BZ.kb = _.mg({
            token: BZ,
            Qa: BZ.va
        });
        var fsb;
        fsb = ["accountSwitcherPlaceholder"];
        _.CZ = class {
            constructor() {
                this.Gma = _.T(!1);
                this.pO = _.T(!1);
                this.Ca = _.r(_.Rh);
                this.u_ = _.r(_.yK);
                this.ka = _.r(_.PJ);
                this.Da = _.r(_.NH);
                this.ha = _.r(BZ);
                this.u8 = _.ql("accountSwitcherPlaceholder");
                this.RZ = _.X( () => {
                    var a;
                    return (a = this.Da.BC()) != null ? a : dsb(this.ka.ha())
                }
                );
                this.ZHa = _.X( () => this.RZ() !== void 0);
                _.Sh( () => {
                    var a, b = (a = this.u8()) == null ? void 0 : a.Ja;
                    b && this.ha.attach(b)
                }
                );
                this.Ca.mc( () => {
                    var a, b = (a = this.u8()) == null ? void 0 : a.Ja;
                    b && this.ha.detach(b)
                }
                )
            }
        }
        ;
        _.CZ.va = function(a) {
            return new (a || _.CZ)
        }
        ;
        _.CZ.Ma = _.z({
            type: _.CZ,
            Fa: [["flow-header-user-icon"]],
            qc: function(a, b) {
                a & 1 && _.Vk(b.u8, fsb, 5);
                a & 2 && _.Wk()
            },
            nc: 2,
            Ob: function(a, b) {
                a & 2 && _.Q("hide-tier-chip-mobile", b.Gma())
            },
            inputs: {
                Gma: [1, "isTierChipHiddenOnMobile"],
                pO: [1, "isPreviewLabelShown"]
            },
            Ka: 4,
            La: 3,
            Na: () => [["accountSwitcherPlaceholder", ""], ["role", "button", "tabindex", "0", "aria-label", "Th\u00f4ng tin v\u1ec1 t\u00e0i kho\u1ea3n", 1, "header-user-button", 3, "click", "keydown.enter", "keydown.space"], [3, "isPreviewLabelShown"], [1, "account-switcher-placeholder"]],
            template: function(a, b) {
                a & 1 && (_.I(0, "div", 1),
                _.M("click", function(c) {
                    c.preventDefault();
                    b.u_.open()
                })("keydown.enter", function(c) {
                    c.preventDefault();
                    b.u_.open()
                })("keydown.space", function(c) {
                    c.preventDefault();
                    b.u_.open()
                }),
                _.L(1, "flow-user-tier-chip", 2)(2, "div", 3, 0),
                _.K());
                a & 2 && (_.Q("has-tier", b.ZHa()),
                _.B(),
                _.H("isPreviewLabelShown", b.pO()))
            },
            dependencies: [AZ],
            styles: ['[_ngcontent-%COMP%]:root{--%NS%glass-raised-box-shadow:0 16px 32px -8px rgba(0,0,0,.4);--%NS%flow-panel-border-radius:20px}.blurred-backdrop[_ngcontent-%COMP%]{background-color:rgba(0,0,0,.32);backdrop-filter:blur(8px);-webkit-backdrop-filter:blur(8px)}mat-icon[_ngcontent-%COMP%]{font-variation-settings:"FILL" var(--%NS%flow-icon-fill,0),"wght" var(--%NS%flow-icon-wght,400)}mat-icon.flow-icon-s[_ngcontent-%COMP%]{font-size:1rem;height:1rem;width:1rem;line-height:1rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-m[_ngcontent-%COMP%]{font-size:1.125rem;height:1.125rem;width:1.125rem;line-height:1.125rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-l[_ngcontent-%COMP%]{font-size:1.25rem;height:1.25rem;width:1.25rem;line-height:1.25rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-xl[_ngcontent-%COMP%]{font-size:1.5rem;height:1.5rem;width:1.5rem;line-height:1.5rem;--%NS%flow-icon-wght:300}mat-icon.fill[_ngcontent-%COMP%]{--%NS%flow-icon-fill:1}@-webkit-keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@-webkit-keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@-webkit-keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}@keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}[_nghost-%COMP%]{display:-webkit-inline-box;display:-webkit-inline-flex;display:-moz-inline-box;display:-ms-inline-flexbox;display:inline-flex}@media (max-width:600px){.hide-tier-chip-mobile[_nghost-%COMP%]{--%NS%flow-user-tier-chip-display:none;--%NS%header-user-button-height:auto;--%NS%header-user-button-padding:0;--%NS%header-user-button-gap:0;--%NS%header-user-button-border:none;--%NS%header-user-button-background:transparent}}.header-user-button[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:var(--%NS%header-user-button-gap,.5rem);padding-inline:var(--%NS%header-user-button-padding,.5rem);padding-block:var(--%NS%header-user-button-padding,.5rem);border:var(--%NS%header-user-button-border,none);background:var(--%NS%header-user-button-background,transparent);color:inherit;font:inherit;cursor:pointer}.header-user-button.has-tier[_ngcontent-%COMP%]{height:var(--%NS%header-user-button-height,3rem);border-radius:16px;border:var(--%NS%header-user-button-border,1px solid var(--%NS%color-white-05));background-color:var(--%NS%header-user-button-background,var(--%NS%background-state-50))}.account-switcher-placeholder[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;-webkit-box-pack:center;-webkit-justify-content:center;-moz-box-pack:center;-ms-flex-pack:center;justify-content:center;width:var(--%NS%ogb-account-switcher-block-size);height:var(--%NS%ogb-account-switcher-block-size);-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0;pointer-events:none}']
        });
        var gsb = function(a, b) {
            if (a & 1) {
                let c = _.t();
                _.I(0, "flow-menu-item", 5);
                _.M("itemClick", function() {
                    var d = _.v(c).Ea;
                    return _.w(d.action == null ? null : d.action())
                });
                _.K()
            }
            a & 2 && (a = b.Ea,
            _.H("label", a.label)("icon", a.type === "icon" ? a.icon : void 0)("iconUrl", a.type === "image" ? a.wk : void 0)("link", a.link))
        }
          , isb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "flow-menu-item", 6);
                _.M("itemClick", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(hsb(c))
                });
                _.I(1, "flow-menu-item-actions")(2, "mat-slide-toggle", 7);
                _.M("change", function(c) {
                    _.v(b);
                    var d = _.O();
                    c.source.checked = d.rI();
                    hsb(d);
                    return _.w()
                });
                _.K()()()
            }
            a & 2 && (a = _.O(),
            _.H("label", a.awa)("icon", "trending_up")("role", "menuitemcheckbox")("ariaChecked", a.rI())("keepOpen", !0),
            _.B(2),
            _.H("hideIcon", !0)("checked", a.rI()))
        }
          , jsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.L(0, "mat-divider");
                _.I(1, "flow-menu-item", 8);
                _.M("itemClick", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(c.dU())
                });
                _.K()
            }
            a & 2 && (a = _.O(),
            _.B(),
            _.H("label", a.xva)("icon", "delete")("disabled", a.GJa()))
        }
          , lsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "button", 2, 0);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(ksb(c))
                });
                _.I(2, "mat-icon");
                _.R(3, "redeem");
                _.K();
                _.L(4, "span", 3);
                _.K()
            }
            a & 2 && (a = _.O(),
            _.H("variant", "transparent")("size", "large"),
            _.D("aria-label", a.rca),
            _.H("matTooltip", a.rca))
        }
          , msb = function(a) {
            a & 1 && _.L(0, "cloud-badge", 5);
            a & 2 && _.H("type", "preview")
        }
          , osb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "a", 12);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O(2);
                    return _.w(nsb(c))
                })("auxclick", function(c) {
                    _.v(b);
                    var d = _.O(2);
                    c.button === 1 && nsb(d);
                    return _.w()
                });
                _.L(1, "img", 13);
                _.I(2, "span");
                _.R(3);
                _.K()()
            }
            a & 2 && (a = _.O(2),
            _.H("variant", "primary")("href", a.xua, _.Di),
            _.B(),
            _.H("src", a.wua),
            _.B(2),
            _.S(a.Tva))
        }
          , qsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "a", 14);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O(2);
                    return _.w(psb(c))
                })("auxclick", function(c) {
                    _.v(b);
                    var d = _.O(2);
                    c.button === 1 && psb(d);
                    return _.w()
                });
                _.I(1, "mat-icon", 15);
                _.R(2, "tv");
                _.K();
                _.I(3, "span");
                _.R(4);
                _.K()()
            }
            a & 2 && (a = _.O(2),
            _.H("variant", "primary")("href", a.yua, _.Di),
            _.B(4),
            _.S(a.Uva))
        }
          , rsb = function(a) {
            a & 1 && (_.E(0, osb, 4, 4, "a", 10),
            _.E(1, qsb, 5, 3, "a", 11));
            a & 2 && (a = _.O(),
            _.F(a.BA() ? 0 : -1),
            _.B(),
            _.F(a.Qm() ? 1 : -1))
        }
          , ssb = function(a, b) {
            if (a & 1) {
                let c = _.t();
                _.I(0, "a", 18);
                _.M("click", function() {
                    var d = _.v(c).Ea
                      , e = _.O(3);
                    return _.w(e.MW(d.event))
                })("auxclick", function(d) {
                    var e = _.v(c).Ea
                      , f = _.O(3);
                    d.button === 1 && f.MW(e.event);
                    return _.w()
                });
                _.L(1, "mat-icon", 19);
                _.K()
            }
            a & 2 && (a = b.Ea,
            _.H("href", a.href, _.Di),
            _.D("aria-label", a.ariaLabel),
            _.B(),
            _.cl(a.d5),
            _.H("svgIcon", a.kv))
        }
          , usb = function(a) {
            a & 1 && _.ik(0, ssb, 2, 5, "a", 17, tsb);
            a & 2 && (a = _.O(2),
            _.jk(a.o$))
        }
          , vsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "button", 20);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O(2);
                    return _.w(c.zP())
                });
                _.I(1, "mat-icon");
                _.R(2, "help");
                _.K()()
            }
            a & 2 && _.H("variant", "transparent")("size", "large")
        }
          , wsb = function(a) {
            a & 1 && (_.E(0, usb, 2, 0),
            _.E(1, vsb, 3, 2, "button", 16));
            a & 2 && (a = _.O(),
            _.F(a.Sx() ? 0 : -1),
            _.B(),
            _.F(a.Ht() ? 1 : -1))
        }
          , xsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "button", 21);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(c.DQ())
                });
                _.I(1, "mat-icon", 22);
                _.R(2, "login");
                _.K();
                _.I(3, "span");
                _.Bk(4, 0);
                _.K()()
            }
            a & 2 && _.H("variant", "secondary")
        }
          , ysb = function(a) {
            a & 1 && _.L(0, "flow-g1-promo-present-icon")
        }
          , zsb = function(a) {
            a & 1 && (_.E(0, ysb, 1, 0, "flow-g1-promo-present-icon"),
            _.L(1, "flow-header-user-icon"));
            a & 2 && (a = _.O(),
            _.F(a.lI() ? 0 : -1))
        };
        _.fN.prototype.dU = _.ca(133, function() {
            var a = this;
            return _.A(function*() {
                var b = [];
                do {
                    var c = void 0;
                    var d = _.Wu(_.jXa(_.iXa(100), (c = d) == null ? void 0 : _.wc(c)));
                    d = yield a.ha.fetch(_.ZM.Eb(d));
                    b.push(..._.Dt(d, _.YM, 1));
                    d = _.yw(d, 2)
                } while (d && !d.Xe());
                b = b.filter(f => !!f.lc());
                var e = new _.tXa(5);
                c = (yield Promise.allSettled(b.map(f => {
                    var g = f.lc();
                    return e.run( () => _.vXa(a, g, !0))
                }
                ))).filter(f => f.status === "rejected");
                for (let f of c)
                    _.At(f.reason, "warning", "Failed to delete a project during bulk deletion");
                return {
                    P_a: b.length,
                    Nia: c.length
                }
            })
        });
        var Asb = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
          , Bsb = function(a) {
            var b = a.GX();
            if (b) {
                var c;
                (c = a.Ha) == null || _.av(c, "G1_PROMO_PRESENT_ICON_CLICK", {
                    G1_INTERVENTION_ID: b.Ll
                });
                return _.WJ(a, {
                    w5: b,
                    entryPoint: "present_icon"
                })
            }
        }
          , Csb = function(a) {
            return _.fu( () => ({
                mutationKey: ["DELETE_ALL_PROJECTS"],
                mutationFn: () => a.dU(),
                onSuccess: () => {
                    _.$H(a.Bb, {
                        queryKey: _.dN
                    });
                    _.WM(a.Bb, {
                        queryKey: ["PROJECT"]
                    })
                }
                ,
                onError: b => {
                    _.At(b, "warning", "Failed to delete all projects");
                    _.$H(a.Bb, {
                        queryKey: _.dN
                    });
                    _.$H(a.Bb, {
                        queryKey: ["PROJECT"]
                    })
                }
            }))
        }
          , Dsb = [{
            href: "https://discord.com/invite/googlelabs",
            kv: "discord_logo",
            d5: "flow-icon-xl",
            ariaLabel: "\u0110\u01b0\u1eddng li\u00ean k\u1ebft \u0111\u1ebfn m\u00e1y ch\u1ee7 Discord c\u1ee7a Google Labs",
            event: "OPEN_DISCORD"
        }, {
            href: "https://www.instagram.com/googleflow",
            kv: "instagram_logo",
            d5: "flow-icon-xl",
            ariaLabel: "\u0110\u01b0\u1eddng li\u00ean k\u1ebft \u0111\u1ebfn Flow tr\u00ean Instagram",
            event: "OPEN_INSTAGRAM"
        }, {
            href: "https://x.com/FlowbyGoogle",
            kv: "x_logo",
            d5: "flow-icon-m",
            ariaLabel: "\u0110\u01b0\u1eddng li\u00ean k\u1ebft \u0111\u1ebfn Flow tr\u00ean X (Twitter)",
            event: "OPEN_TWITTER"
        }]
          , Esb = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ib() {
                return _.Y(this, _.Z, 2)
            }
            Hb() {
                return _.kv(this, _.Z, 2)
            }
            getName() {
                return _.p(this, 1)
            }
        }
          , Fsb = class extends _.n {
            constructor(a) {
                super(a)
            }
        }
          , Gsb = {
            Xa: "xI9TVb",
            Za: "/AiSandbox.GetUserPreferences",
            hb: !0,
            Ya: Fsb,
            lb: Esb
        };
        Gsb[_.Hy] = !0;
        var Hsb = new _.Yy(Gsb)
          , Isb = class extends _.n {
            constructor(a) {
                super(a)
            }
            Ib() {
                return _.Y(this, _.Z, 3)
            }
            Hb() {
                return _.kv(this, _.Z, 3)
            }
        }
          , Jsb = {
            Xa: "mu42vc",
            Za: "/AiSandbox.UpdateUserPreferences",
            hb: !1,
            Ya: Fsb,
            lb: Isb
        };
        Jsb[_.Hy] = !0;
        var Ksb = new _.Yy(Jsb)
          , Lsb = {
            preview: "B\u1ea3n xem tr\u01b0\u1edbc",
            "new": "M\u1edbi"
        }
          , DZ = class {
            constructor() {
                this.type = _.pl.required();
                this.label = _.X( () => Lsb[this.type()])
            }
        }
        ;
        DZ.va = function(a) {
            return new (a || DZ)
        }
        ;
        DZ.Ma = _.z({
            type: DZ,
            Fa: [["cloud-badge"]],
            inputs: {
                type: [1, "type"]
            },
            Ka: 2,
            La: 3,
            Na: [[1, "cloud-badge"]],
            template: function(a, b) {
                a & 1 && (_.lk(0, "span", 0),
                _.R(1),
                _.mk());
                a & 2 && (_.Q("cloud-badge-new", b.type() === "new"),
                _.B(),
                _.S(b.label()))
            },
            styles: ["[_nghost-%COMP%]{display:-webkit-inline-box;display:-webkit-inline-flex;display:-moz-inline-box;display:-ms-inline-flexbox;display:inline-flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center}.cloud-badge[_ngcontent-%COMP%]{border-radius:4px;padding:.125rem .25rem;font:var(--%NS%mat-sys-label-medium);background-color:var(--%NS%foreground-state-15);display:inline-block;white-space:nowrap}.cloud-badge-new[_ngcontent-%COMP%]{background-color:var(--%NS%mat-sys-primary);color:var(--%NS%mat-sys-on-primary)}"]
        });
        var Msb = ["USER_PREFERENCES"]
          , Nsb = function(a, b) {
            return _.fu( () => ({
                mutationKey: ["UPDATE_USER_PREFERENCES"],
                mutationFn: c => _.A(function*() {
                    var d = new Isb;
                    var e = b(c);
                    d = _.Lp(d, 1, e);
                    e = new _.hz;
                    e = _.Xc(e, 1, _.Sb, "enable_product_improvement", void 0, _.Ub);
                    d = _.Lp(d, 2, e);
                    d = _.Cc(d);
                    return a.ha.mutate(Ksb.Eb(d))
                }),
                onSuccess: c => {
                    _.YH(a.Bb, Msb, () => c)
                }
                ,
                onError: () => {
                    _.$H(a.Bb, {
                        queryKey: Msb
                    })
                }
            }))
        }
          , Osb = function(a) {
            return Nsb(a, b => {
                var c, d, e, f, g = new Fsb;
                b = _.Do(g, 3, b);
                g = (e = (c = a.query.data()) == null ? void 0 : _.Jw(c, 1)) != null ? e : !0;
                c = _.sv(b, 1, g);
                e = (f = (d = a.query.data()) == null ? void 0 : _.Jw(d, 2)) != null ? f : !0;
                d = _.sv(c, 2, e);
                return _.Cc(d)
            }
            )
        }
          , EZ = class {
            constructor() {
                this.ha = _.r(_.OF);
                this.Bb = _.r(_.bu);
                _.r(_.wy);
                this.ka = _.r(_.OH);
                this.ie = _.st();
                this.query = _.hu( () => ({
                    queryKey: Msb,
                    enabled: this.ie && this.ka.ha(),
                    queryFn: () => {
                        var a = this;
                        return _.A(function*() {
                            var b = a.ha
                              , c = b.fetch
                              , d = Hsb.Eb;
                            var e = new Esb;
                            e = _.Rw(e, 1, "userPreferences/");
                            return yield c.call(b, d.call(Hsb, _.Cc(e)))
                        })
                    }
                }));
                this.rI = _.X( () => {
                    var a;
                    return (a = this.query.data()) == null ? void 0 : _.Jw(a, 3)
                }
                );
                this.isPending = _.X( () => this.query.isPending());
                this.error = _.X( () => this.query.error())
            }
        }
        ;
        EZ.va = function(a) {
            return new (a || EZ)
        }
        ;
        EZ.kb = _.mg({
            token: EZ,
            Qa: EZ.va
        });
        var Psb = (a, b) => b.label
          , Qsb = new _.dv("{COUNT,plural, =1{Kh\u00f4ng xo\u00e1 \u0111\u01b0\u1ee3c 1 d\u1ef1 \u00e1n}other{Kh\u00f4ng xo\u00e1 \u0111\u01b0\u1ee3c # d\u1ef1 \u00e1n}}")
          , hsb = function(a) {
            FZ(a, "HELP_IMPROVE");
            var b = a.rI();
            a.jc.open(_.ZN, {
                data: {
                    title: b ? "T\u1eaft t\u00ednh n\u0103ng c\u1ea3i thi\u1ec7n s\u1ea3n ph\u1ea9m?" : "B\u1eadt t\u00ednh n\u0103ng c\u1ea3i thi\u1ec7n s\u1ea3n ph\u1ea9m?",
                    message: b ? "N\u1ed9i dung b\u1ea1n t\u1ea1o s\u1ebd kh\u00f4ng c\u00f2n \u0111\u01b0\u1ee3c d\u00f9ng \u0111\u1ec3 c\u1ea3i thi\u1ec7n Flow c\u0169ng nh\u01b0 c\u00e1c s\u1ea3n ph\u1ea9m v\u00e0 d\u1ecbch v\u1ee5 c\u1ee7a Google, k\u1ec3 c\u1ea3 c\u00e1c m\u00f4 h\u00ecnh AI." : "N\u1ed9i dung b\u1ea1n t\u1ea1o s\u1ebd \u0111\u01b0\u1ee3c d\u00f9ng \u0111\u1ec3 c\u1ea3i thi\u1ec7n Flow c\u0169ng nh\u01b0 c\u00e1c s\u1ea3n ph\u1ea9m v\u00e0 d\u1ecbch v\u1ee5 c\u1ee7a Google, k\u1ec3 c\u1ea3 c\u00e1c m\u00f4 h\u00ecnh AI.",
                    Mk: b ? "T\u1eaft" : "B\u1eadt",
                    eq: b,
                    Aq: () => _.A(function*() {
                        try {
                            yield a.Ia.je(!b)
                        } catch (c) {
                            throw _.HI(a.Ua, _.JI, {
                                data: {
                                    message: "\u0110\u00e3 x\u1ea3y ra l\u1ed7i.",
                                    type: "error"
                                }
                            }),
                            c;
                        }
                    })
                },
                Gd: "blurred-backdrop",
                ql: !1
            })
        }
          , FZ = function(a, b) {
            _.av(a.Ha, "FLOW_HEADER", {
                FLOW_HEADER_ACTION: b
            })
        }
          , Rsb = function(a, b, c="standard") {
            _.HI(a.Ua, _.JI, {
                data: {
                    message: b,
                    type: c,
                    fl: !0
                }
            })
        }
          , GZ = class {
            constructor() {
                this.Rx = _.T(!1);
                this.awa = "Gi\u00fap c\u1ea3i thi\u1ec7n Flow";
                this.xva = "Xo\u00e1 t\u1ea5t c\u1ea3 c\u00e1c d\u1ef1 \u00e1n";
                this.oa = _.r(_.zZ);
                this.Da = _.r(_.NH);
                this.ha = _.r(_.OH);
                this.Dj = _.r(_.sK);
                this.Ha = _.r(_.UJ);
                this.jc = _.r(_.$I);
                this.Ua = _.r(_.II);
                this.ma = _.r(EZ);
                this.Aa = _.r(_.VJ);
                this.ka = _.r(_.fN);
                this.ua = {
                    type: "image",
                    label: "Flow Music",
                    wk: _.bI,
                    link: {
                        href: "https://www.flowmusic.app/?utm_source=flow"
                    },
                    action: () => {
                        FZ(this, "OPEN_FLOW_MUSIC")
                    }
                };
                this.na = {
                    type: "icon",
                    label: "Gi\u1edbi thi\u1ec7u v\u1ec1 Flow",
                    icon: "info",
                    link: {
                        href: "/flow",
                        target: "_blank"
                    },
                    action: () => {
                        FZ(this, "ABOUT_FLOW")
                    }
                };
                this.wa = {
                    type: "icon",
                    label: "\u0110i\u1ec1u kho\u1ea3n d\u1ecbch v\u1ee5",
                    icon: "gavel",
                    link: {
                        href: this.Dj.PB
                    },
                    action: () => {
                        FZ(this, "TERMS_OF_SERVICE")
                    }
                };
                this.eP = _.X( () => [this.Da.BA() ? this.ua : null, this.Da.ha.oa() ? this.na : null, {
                    type: "icon",
                    label: "T\u00ecm hi\u1ec3u v\u1ec1 Flow",
                    icon: "smart_display",
                    link: {
                        href: "https://www.youtube.com/watch?v=oKjDeMtBZ4g"
                    },
                    action: () => {
                        FZ(this, "LEARN_MORE")
                    }
                }, {
                    type: "icon",
                    label: "G\u1eedi \u00fd ki\u1ebfn ph\u1ea3n h\u1ed3i v\u1ec1 \u1ee9ng d\u1ee5ng",
                    icon: "feedback",
                    action: () => {
                        FZ(this, "SEND_FEEDBACK");
                        this.oa.jo("PROJECTS_PAGE_THREE_DOT_MENU")
                    }
                }, {
                    type: "icon",
                    label: "B\u00e1o c\u00e1o v\u1ea5n \u0111\u1ec1 ph\u00e1p l\u00fd",
                    icon: "flag",
                    link: {
                        href: "https://support.google.com/legal/troubleshooter/1114905?uraw=r_20cf4f41bc75e35e"
                    },
                    action: () => {
                        FZ(this, "REPORT_ISSUE")
                    }
                }, {
                    type: "icon",
                    label: "Th\u00f4ng ba\u0301o v\u00ea\u0300 quy\u00ea\u0300n ri\u00eang t\u01b0",
                    icon: "info",
                    link: {
                        href: this.Dj.RP
                    },
                    action: () => {
                        FZ(this, "PRIVACY_NOTICE")
                    }
                }, this.Da.Vx() ? this.wa : null].filter(_.vu));
                this.Ba = this.Aa.Zb( () => ({
                    ju: 22,
                    enabled: !this.Rx() && this.Da.sI() && this.ha.ha()
                }));
                this.Ia = Osb(this.ma);
                this.Ga = Csb(this.ka);
                this.Bd = this.ka.Zb( () => ({
                    enabled: !this.Rx() && this.ha.ha()
                }));
                this.HJa = _.X( () => !this.Rx() && this.ha.ha());
                this.GJa = _.X( () => {
                    var a, b = (a = this.Bd.data()) == null ? void 0 : a.pages;
                    return b ? b.every(c => c.Bd.length === 0) : !0
                }
                );
                this.rI = _.X( () => {
                    var a;
                    return (a = this.ma.rI()) != null ? a : !1
                }
                );
                this.sI = _.X( () => {
                    if (!this.Da.sI())
                        return !1;
                    var a = this.Ba.data();
                    return a ? !_.Dt(a, Asb, 2).some(b => _.lu(b, 1) === 16) : !1
                }
                )
            }
            dU() {
                this.jc.open(_.ZN, {
                    data: {
                        title: "Xo\u00e1 t\u1ea5t c\u1ea3 c\u00e1c d\u1ef1 \u00e1n",
                        message: "T\u1ea5t c\u1ea3 d\u1ef1 \u00e1n v\u00e0 n\u1ed9i dung c\u1ee7a d\u1ef1 \u00e1n s\u1ebd b\u1ecb xo\u00e1 v\u0129nh vi\u1ec5n. B\u1ea1n s\u1ebd kh\u00f4ng th\u1ec3 hu\u1ef7 thao t\u00e1c n\u00e0y sau khi th\u1ef1c hi\u1ec7n.",
                        Mk: "Xo\u00e1 t\u1ea5t c\u1ea3",
                        Aq: () => {
                            var a = this;
                            return _.A(function*() {
                                FZ(a, "DELETE_ALL_PROJECTS");
                                try {
                                    let b = yield a.Ga.je();
                                    b.Nia > 0 ? Rsb(a, Qsb.format({
                                        COUNT: b.Nia
                                    }), "error") : Rsb(a, "\u0110\u00e3 xo\u00e1 th\u00e0nh c\u00f4ng t\u1ea5t c\u1ea3 d\u1ef1 \u00e1n", "success")
                                } catch (b) {
                                    throw Rsb(a, "Kh\u00f4ng xo\u00e1 \u0111\u01b0\u1ee3c d\u1ef1 \u00e1n", "error"),
                                    b;
                                }
                            })
                        }
                        ,
                        eq: !0
                    },
                    Gd: "blurred-backdrop"
                })
            }
        }
        ;
        GZ.va = function(a) {
            return new (a || GZ)
        }
        ;
        GZ.Ma = _.z({
            type: GZ,
            Fa: [["flow-more-options-menu"]],
            inputs: {
                Rx: [1, "isSignedOut"]
            },
            Ka: 9,
            La: 5,
            Na: () => [["menu", "matMenu"], ["mat-icon-button", "", "flow-icon-button", "", "aria-label", "L\u1ef1a ch\u1ecdn kh\u00e1c", "matTooltip", "L\u1ef1a ch\u1ecdn kh\u00e1c", 1, "more-options-trigger", 3, "variant", "size", "matMenuTriggerFor"], [1, "flow-menu-panel-wide", "projects-page-more-options-panel"], [3, "label", "icon", "iconUrl", "link"], [3, "label", "icon", "role", "ariaChecked", "keepOpen"], [3, "itemClick", "label", "icon", "iconUrl", "link"], [3, "itemClick", "label", "icon", "role", "ariaChecked", "keepOpen"], ["aria-hidden", "true", "tabindex", "-1", 1, "help-improve-toggle", 3, "change", "hideIcon", "checked"], [1, "delete-button", 3, "itemClick", "label", "icon", "disabled"]],
            template: function(a, b) {
                a & 1 && (_.I(0, "button", 1)(1, "mat-icon"),
                _.R(2, "more_vert"),
                _.K()(),
                _.I(3, "mat-menu", 2, 0),
                _.ik(5, gsb, 1, 4, "flow-menu-item", 3, Psb),
                _.E(7, isb, 3, 7, "flow-menu-item", 4),
                _.E(8, jsb, 2, 3),
                _.K());
                a & 2 && (a = _.Xk(4),
                _.H("variant", "transparent")("size", "large")("matMenuTriggerFor", a),
                _.B(5),
                _.jk(b.eP()),
                _.B(2),
                _.F(!b.Rx() && b.sI() ? 7 : -1),
                _.B(),
                _.F(b.HJa() ? 8 : -1))
            },
            dependencies: [_.MI, _.aO, _.jI, _.DH, _.dV, _.hV, _.kK, _.hJ, _.iV, _.lV],
            styles: ["button.more-options-trigger[_ngcontent-%COMP%]{--%NS%mat-icon-button-icon-size:1.5rem;--%NS%mat-icon-button-container-shape:999px;--%NS%mat-icon-button-state-layer-size:2.75rem;--%NS%mat-icon-button-icon-color:var(--%NS%foreground-neutral-000);--%NS%mat-icon-button-state-layer-color:var(--%NS%foreground-state-15);--%NS%mat-icon-button-hover-state-layer-opacity:1;--%NS%mat-icon-button-pressed-state-layer-opacity:1;--%NS%mat-icon-button-focus-state-layer-opacity:1;--%NS%mat-icon-button-ripple-color:transparent}.delete-button[_ngcontent-%COMP%]{--%NS%mat-menu-item-label-text-color:var(--%NS%foreground-color-danger);--%NS%mat-menu-item-icon-color:var(--%NS%foreground-color-danger)}"]
        });
        var Ssb = ["presentIconButton"]
          , ksb = function(a) {
            var b;
            (b = Bsb(a.ha)) == null || _.WI(b).Ra(_.Au(a.Ca)).subscribe(c => {
                c !== "claimed" && _.Sh( () => {
                    var d;
                    (d = a.button()) == null || d.Ja.focus()
                }
                , {
                    Mb: a.Mb
                })
            }
            )
        }
          , HZ = class {
            constructor() {
                this.ha = _.r(_.XJ);
                this.Ca = _.r(_.Rh);
                this.Mb = _.r(_.Nh);
                this.button = _.ql("presentIconButton", Object.assign({}, {}, {
                    read: _.vh
                }));
                this.GX = this.ha.GX;
                this.rca = "Xem \u01b0u \u0111\u00e3i b\u1ea1n ch\u01b0a nh\u1eadn"
            }
        }
        ;
        HZ.va = function(a) {
            return new (a || HZ)
        }
        ;
        HZ.Ma = _.z({
            type: HZ,
            Fa: [["flow-g1-promo-present-icon"]],
            qc: function(a, b) {
                a & 1 && _.Vk(b.button, Ssb, 5, _.vh);
                a & 2 && _.Wk()
            },
            Ka: 1,
            La: 1,
            Na: [["presentIconButton", ""], ["matIconButton", "", "flow-icon-button", "", 1, "present-icon-button", 3, "variant", "size", "aria-label", "matTooltip"], ["matIconButton", "", "flow-icon-button", "", 1, "present-icon-button", 3, "click", "variant", "size", "aria-label", "matTooltip"], ["aria-hidden", "true", 1, "present-icon-badge"]],
            template: function(a, b) {
                a & 1 && _.E(0, lsb, 5, 4, "button", 1);
                a & 2 && _.F(b.GX() ? 0 : -1)
            },
            dependencies: [_.MI, _.jI, _.DH, _.hJ],
            styles: ["[_nghost-%COMP%]{display:contents}.present-icon-button[_ngcontent-%COMP%]{position:relative}.present-icon-badge[_ngcontent-%COMP%]{position:absolute;inset-block-start:.375rem;inset-inline-end:.375rem;width:.5rem;height:.5rem;border-radius:50%;background-color:var(--%NS%background-color-danger);pointer-events:none}"]
        });
        var tsb, nsb, psb;
        tsb = (a, b) => b.kv;
        nsb = function(a) {
            var b;
            (b = a.Ha) == null || _.av(b, "OPEN_FLOW_MUSIC")
        }
        ;
        psb = function(a) {
            var b;
            (b = a.Ha) == null || _.av(b, "OPEN_FLOW_TV")
        }
        ;
        _.IZ = class {
            constructor() {
                this.Rx = _.T(!1);
                this.document = _.r(_.Rs);
                this.tb = _.r(_.TD);
                this.ha = _.r(_.iI);
                this.Da = _.r(_.NH);
                this.ka = _.r(_.XJ);
                this.ma = _.r(_.wZ);
                this.Ha = _.r(_.UJ, {
                    optional: !0
                });
                this.Pb = _.Uu();
                this.BA = _.X( () => this.Da.BA());
                this.Qm = _.X( () => this.Da.Qm());
                this.Sx = _.X( () => this.Da.Sx());
                this.Ht = _.X( () => this.Da.Ht());
                this.nSa = _.X( () => this.Da.ha.Ga());
                this.lI = _.X( () => this.ka.isEnabled);
                this.o$ = Dsb;
                this.Tva = "Flow Music";
                this.Uva = "Flow TV";
                this.xua = "https://www.flowmusic.app/?utm_source=flow";
                this.wua = _.bI;
                this.yua = "https://labs.google/flow/tv";
                _.hI(this.ha, "discord_logo", _.$rb);
                _.hI(this.ha, "instagram_logo", _.asb);
                _.hI(this.ha, "x_logo", _.bsb)
            }
            zP() {
                this.ma.zP()
            }
            MW(a) {
                var b;
                (b = this.Ha) == null || _.av(b, a)
            }
            DQ() {
                var a, b = ((a = this.document.defaultView) == null ? void 0 : a.navigator.userAgent) || "";
                _.xu(b) ? this.tb.navigate(["/unsupported-browser"], {
                    Ic: {
                        returnUrl: this.document.location.href
                    }
                }) : (a = new URL("https://accounts.google.com/ServiceLogin"),
                a.searchParams.set("continue", this.document.location.href),
                (a = _.Yl(a.toString())) ? _.sm(this.document.location, a) : (_.At(Error("Hh"), "warning"),
                (a = _.Yl("https://accounts.google.com/ServiceLogin")) && _.sm(this.document.location, a)))
            }
        }
        ;
        _.IZ.va = function(a) {
            return new (a || _.IZ)
        }
        ;
        _.IZ.Ma = _.z({
            type: _.IZ,
            Fa: [["flow-app-header"]],
            nc: 2,
            Ob: function(a, b) {
                a & 2 && _.Q("header-mobile", b.Pb())
            },
            inputs: {
                Rx: [1, "isSignedOut"]
            },
            Ka: 13,
            La: 8,
            Na: () => ["\u0110\u0103ng nh\u1eadp", [1, "flow-header"], [1, "header-backdrop"], [1, "header-title"], [1, "header-title-link", 3, "routerLink"], [3, "type"], [1, "header-right"], [1, "header-actions"], [3, "isSignedOut"], ["mat-button", "", "flow-button", "", 1, "header-signin-button", 3, "variant"], ["mat-button", "", "flow-button", "", "target", "_blank", "rel", "noopener noreferrer", 1, "header-music-button", 3, "variant", "href"], ["mat-button", "", "flow-button", "", "target", "_blank", "rel", "noopener noreferrer", 1, "header-tv-button", 3, "variant", "href"], ["mat-button", "", "flow-button", "", "target", "_blank", "rel", "noopener noreferrer", 1, "header-music-button", 3, "click", "auxclick", "variant", "href"], ["alt", "", "width", "20", "height", "20", 1, "flow-music-icon", 3, "src"], ["mat-button", "", "flow-button", "", "target", "_blank", "rel", "noopener noreferrer", 1, "header-tv-button", 3, "click", "auxclick", "variant", "href"], [1, "flow-icon-l"], ["mat-icon-button", "", "flow-icon-button", "", "matTooltip", "Tr\u1ee3 gi\u00fap v\u1ec1 s\u1ea3n ph\u1ea9m", "aria-label", "Tr\u1ee3 gi\u00fap v\u1ec1 s\u1ea3n ph\u1ea9m", 1, "header-help-button", 3, "variant", "size"], ["target", "_blank", "rel", "noopener noreferrer", 1, "header-social-link", 3, "href", "aria-label"], ["target", "_blank", "rel", "noopener noreferrer", 1, "header-social-link", 3, "click", "auxclick", "href", "aria-label"], [3, "svgIcon"], ["mat-icon-button", "", "flow-icon-button", "", "matTooltip", "Tr\u1ee3 gi\u00fap v\u1ec1 s\u1ea3n ph\u1ea9m", "aria-label", "Tr\u1ee3 gi\u00fap v\u1ec1 s\u1ea3n ph\u1ea9m", 1, "header-help-button", 3, "click", "variant", "size"], ["mat-button", "", "flow-button", "", 1, "header-signin-button", 3, "click", "variant"], [1, "mat-icon-rtl-mirror"]],
            template: function(a, b) {
                a & 1 && (_.I(0, "header", 1),
                _.L(1, "div", 2),
                _.I(2, "h1", 3)(3, "a", 4),
                _.R(4, "Google Flow"),
                _.K(),
                _.E(5, msb, 1, 1, "cloud-badge", 5),
                _.K(),
                _.I(6, "div", 6),
                _.E(7, rsb, 2, 2),
                _.I(8, "div", 7),
                _.E(9, wsb, 2, 2),
                _.L(10, "flow-more-options-menu", 8),
                _.K(),
                _.E(11, xsb, 5, 1, "button", 9)(12, zsb, 2, 1),
                _.K()());
                a & 2 && (_.Q("header-mobile", b.Pb()),
                _.B(3),
                _.H("routerLink", "/"),
                _.B(2),
                _.F(b.nSa() ? 5 : -1),
                _.B(2),
                _.F(b.Pb() ? -1 : 7),
                _.B(2),
                _.F(b.Pb() ? -1 : 9),
                _.B(),
                _.H("isSignedOut", b.Rx()),
                _.B(),
                _.F(b.Rx() ? 11 : 12))
            },
            dependencies: [DZ, _.EH, _.MI, GZ, HZ, _.CZ, _.BH, _.jI, _.DH, _.hJ, _.VD],
            styles: ['[_ngcontent-%COMP%]:root{--%NS%glass-raised-box-shadow:0 16px 32px -8px rgba(0,0,0,.4);--%NS%flow-panel-border-radius:20px}.blurred-backdrop[_ngcontent-%COMP%]{background-color:rgba(0,0,0,.32);backdrop-filter:blur(8px);-webkit-backdrop-filter:blur(8px)}mat-icon[_ngcontent-%COMP%]{font-variation-settings:"FILL" var(--%NS%flow-icon-fill,0),"wght" var(--%NS%flow-icon-wght,400)}mat-icon.flow-icon-s[_ngcontent-%COMP%]{font-size:1rem;height:1rem;width:1rem;line-height:1rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-m[_ngcontent-%COMP%]{font-size:1.125rem;height:1.125rem;width:1.125rem;line-height:1.125rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-l[_ngcontent-%COMP%]{font-size:1.25rem;height:1.25rem;width:1.25rem;line-height:1.25rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-xl[_ngcontent-%COMP%]{font-size:1.5rem;height:1.5rem;width:1.5rem;line-height:1.5rem;--%NS%flow-icon-wght:300}mat-icon.fill[_ngcontent-%COMP%]{--%NS%flow-icon-fill:1}@-webkit-keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@-webkit-keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@-webkit-keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}@keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}[_nghost-%COMP%]{display:block;width:100%}.flow-header[_ngcontent-%COMP%]{position:-webkit-sticky;position:sticky;top:0;inset-inline-start:0;display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;-webkit-box-pack:justify;-webkit-justify-content:space-between;-moz-box-pack:justify;-ms-flex-pack:justify;justify-content:space-between;isolation:isolate;width:100%;height:var(--%NS%header-desktop-height);padding-block:1rem;padding-inline:1rem}@media (min-width:600px){.flow-header[_ngcontent-%COMP%]{padding-inline:1.5rem}}.flow-header[_ngcontent-%COMP%]{z-index:1;background:-webkit-gradient(linear,left top,left bottom,color-stop(60%,color-mix(in srgb,var(--%NS%background-neutral-000) 20%,transparent)),to(transparent));background:-webkit-linear-gradient(top,color-mix(in srgb,var(--%NS%background-neutral-000) 20%,transparent) 60%,transparent);background:linear-gradient(180deg,color-mix(in srgb,var(--%NS%background-neutral-000) 20%,transparent) 60%,transparent)}.header-mobile[_ngcontent-%COMP%]{-webkit-padding-end:var(--%NS%page-padding);-moz-padding-end:var(--%NS%page-padding);padding-inline-end:var(--%NS%page-padding)}.header-title[_ngcontent-%COMP%]{font:var(--%NS%mat-sys-title-medium);color:var(--%NS%foreground-neutral-000);margin:0;white-space:nowrap;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}.header-title-link[_ngcontent-%COMP%]{color:inherit;text-decoration:none;white-space:nowrap}cloud-badge[_ngcontent-%COMP%]{-webkit-margin-start:.5rem;-moz-margin-start:.5rem;margin-inline-start:.5rem;vertical-align:middle}.header-right[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.5rem;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}@media (min-width:600px){.header-right[_ngcontent-%COMP%]{gap:1.5rem}}.header-actions[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.25rem}.header-social-link[_ngcontent-%COMP%]{position:relative;display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;-webkit-box-pack:center;-webkit-justify-content:center;-moz-box-pack:center;-ms-flex-pack:center;justify-content:center;color:var(--%NS%foreground-neutral-000);padding-inline:.625rem}.header-social-link[_ngcontent-%COMP%]:after{content:"";position:absolute;top:50%;inset-inline-start:50%;-webkit-transform:translate(-50%,-50%);transform:translate(-50%,-50%);z-index:-1;width:2.75rem;height:2.75rem;border-radius:50%;background-color:var(--%NS%foreground-state-15);opacity:0;-webkit-transition:opacity .2s ease-in-out;transition:opacity .2s ease-in-out}.header-social-link[_ngcontent-%COMP%]:active:after, .header-social-link[_ngcontent-%COMP%]:hover:after{opacity:1}button.header-help-button[_ngcontent-%COMP%]{--%NS%mat-icon-button-icon-size:1.5rem;--%NS%mat-icon-button-container-shape:999px;--%NS%mat-icon-button-state-layer-size:2.75rem;--%NS%mat-icon-button-icon-color:var(--%NS%foreground-neutral-000);--%NS%mat-icon-button-state-layer-color:var(--%NS%foreground-state-15);--%NS%mat-icon-button-hover-state-layer-opacity:1;--%NS%mat-icon-button-pressed-state-layer-opacity:1;--%NS%mat-icon-button-focus-state-layer-opacity:1;--%NS%mat-icon-button-ripple-color:transparent}.header-music-button[_ngcontent-%COMP%], .header-signin-button[_ngcontent-%COMP%], .header-tv-button[_ngcontent-%COMP%]{--%NS%mat-button-filled-label-text-color:var(--%NS%foreground-neutral-100);--%NS%mat-button-filled-label-text-font:var(--%NS%mat-sys-title-small-font);--%NS%mat-button-filled-label-text-size:var(--%NS%mat-sys-title-small-size);--%NS%mat-button-filled-label-text-weight:var(--%NS%mat-sys-title-small-weight);--%NS%mat-button-filled-label-text-tracking:var(--%NS%mat-sys-title-small-tracking);--%NS%mat-button-filled-container-height:2.75rem;--%NS%mat-button-filled-container-shape:999px;--%NS%mat-button-filled-container-color:var(--%NS%color-white-05);--%NS%mat-button-filled-horizontal-padding:1.5rem;--%NS%mat-button-filled-icon-spacing:0.5rem;border:1px solid var(--%NS%color-white-15)}.header-music-button[_ngcontent-%COMP%]:hover, .header-signin-button[_ngcontent-%COMP%]:hover, .header-tv-button[_ngcontent-%COMP%]:hover{--%NS%mat-button-filled-container-color:var(--%NS%color-white-15)}.header-music-button[_ngcontent-%COMP%]     .mdc-button__label, .header-signin-button[_ngcontent-%COMP%]     .mdc-button__label, .header-tv-button[_ngcontent-%COMP%]     .mdc-button__label{display:-webkit-inline-box;display:-webkit-inline-flex;display:-moz-inline-box;display:-ms-inline-flexbox;display:inline-flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.5rem}.header-music-button[_ngcontent-%COMP%]     .mdc-button__label mat-icon, .header-signin-button[_ngcontent-%COMP%]     .mdc-button__label mat-icon, .header-tv-button[_ngcontent-%COMP%]     .mdc-button__label mat-icon{margin-right:0;-webkit-margin-end:0;-moz-margin-end:0;margin-inline-end:0}.flow-music-icon[_ngcontent-%COMP%]{width:1.25rem;height:1.25rem;border-radius:.1875rem;object-fit:cover;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}.header-backdrop[_ngcontent-%COMP%]{position:absolute;inset:0;pointer-events:none;backdrop-filter:blur(2px);-webkit-backdrop-filter:blur(2px);-webkit-mask-image:-webkit-gradient(linear,left top,left bottom,from(#000),color-stop(60%,#000),color-stop(80%,rgba(0,0,0,.8)),color-stop(90%,rgba(0,0,0,.4)),color-stop(95%,rgba(0,0,0,.2)),color-stop(97%,rgba(0,0,0,.1)),to(transparent));-webkit-mask-image:-webkit-linear-gradient(top,#000,#000 60%,rgba(0,0,0,.8) 80%,rgba(0,0,0,.4) 90%,rgba(0,0,0,.2) 95%,rgba(0,0,0,.1) 97%,transparent);mask-image:-webkit-gradient(linear,left top,left bottom,from(#000),color-stop(60%,#000),color-stop(80%,rgba(0,0,0,.8)),color-stop(90%,rgba(0,0,0,.4)),color-stop(95%,rgba(0,0,0,.2)),color-stop(97%,rgba(0,0,0,.1)),to(transparent));mask-image:linear-gradient(180deg,#000 0,#000 60%,rgba(0,0,0,.8) 80%,rgba(0,0,0,.4) 90%,rgba(0,0,0,.2) 95%,rgba(0,0,0,.1) 97%,transparent);z-index:-1}']
        });
        _.Om("zxMy9c");
        var Tsb = function(a) {
            if (a & 1) {
                let b = _.t();
                _.I(0, "div", 0)(1, "div", 1);
                _.Qf();
                _.I(2, "svg", 2);
                _.L(3, "path", 3);
                _.K()();
                _.Rf();
                _.I(4, "div", 4)(5, "span", 5);
                _.R(6, "Google Flow");
                _.K();
                _.I(7, "span", 6);
                _.R(8);
                _.K()();
                _.I(9, "div", 7)(10, "a", 8);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O();
                    _.av(c.Ha, "FLOW_SMART_MOBILE_APP_BANNER", {
                        FLOW_SMART_MOBILE_APP_BANNER_ACTION: "OPEN"
                    });
                    return _.w()
                });
                _.R(11);
                _.K();
                _.I(12, "button", 9);
                _.M("click", function() {
                    _.v(b);
                    var c = _.O();
                    return _.w(c.Xw())
                });
                _.I(13, "mat-icon", 10);
                _.R(14, "close");
                _.K()()()()
            }
            a & 2 && (a = _.O(),
            _.B(8),
            _.il(" ", a.Mxa, " "),
            _.B(2),
            _.H("variant", "primary")("size", "small")("href", a.Ora(), _.Di),
            _.B(),
            _.il(" ", a.Lxa, " "),
            _.B(),
            _.H("variant", "transparent"),
            _.D("aria-label", a.Kxa))
        }
          , Usb = function(a) {
            if (typeof a.document !== "undefined") {
                if (!a.document.querySelector('meta[name="apple-itunes-app"]')) {
                    var b = a.document.createElement("meta");
                    b.name = "apple-itunes-app";
                    b.content = "app-id=6751294549";
                    a.document.head.appendChild(b)
                }
                a.document.querySelector('meta[name="google-play-app"]') || (b = a.document.createElement("meta"),
                b.name = "google-play-app",
                b.content = "app-id=com.google.android.apps.labs.whisk",
                a.document.head.appendChild(b))
            }
        }
          , JZ = class {
            constructor() {
                this.ka = _.Uu();
                this.Ha = _.r(_.UJ);
                this.ma = _.r(_.IH);
                this.document = _.r(_.Rs);
                this.Da = _.r(_.NH);
                this.Mxa = "M\u1edf trong \u1ee9ng d\u1ee5ng Google Flow";
                this.Lxa = "M\u1edf";
                this.Kxa = "\u0110\u00f3ng bi\u1ec3u ng\u1eef";
                this.ha = this.ma.signal(_.HH.Yaa, !1);
                this.Ora = _.X( () => {
                    a: {
                        if (typeof window !== "undefined" && window.navigator) {
                            var a = window.navigator.userAgent || "";
                            var b = window.navigator.maxTouchPoints || 0
                              , c = /Safari/i.test(a) && !/Chrome|CriOS|FxiOS|EdgiOS|Android/i.test(a);
                            if (/iPad|iPhone|iPod/.test(a) || /Macintosh/.test(a) && b > 1) {
                                a = c ? void 0 : "https://apps.apple.com/app/id6751294549";
                                break a
                            }
                            if (/Android/.test(a)) {
                                a = "https://play.google.com/store/apps/details?id=com.google.android.apps.labs.whisk";
                                break a
                            }
                        }
                        a = void 0
                    }
                    return a
                }
                );
                this.isVisible = _.X( () => this.Da.ha.Aa() && this.ka() && !this.ha() && !!this.Ora());
                this.Da.ha.Aa() && this.ka() && Usb(this)
            }
            Xw() {
                _.av(this.Ha, "FLOW_SMART_MOBILE_APP_BANNER", {
                    FLOW_SMART_MOBILE_APP_BANNER_ACTION: "DISMISS"
                });
                this.ha.set(!0)
            }
        }
        ;
        JZ.va = function(a) {
            return new (a || JZ)
        }
        ;
        JZ.Ma = _.z({
            type: JZ,
            Fa: [["flow-smart-app-banner"]],
            Ka: 1,
            La: 1,
            Na: [[1, "smart-app-banner-container"], ["aria-hidden", "true", 1, "smart-app-banner-app-icon"], ["xmlns", "http://www.w3.org/2000/svg", "viewBox", "0 -960 960 960", 1, "smart-app-banner-icon-symbol"], ["fill", "#041E49", "d", "M209-120q-42 0-70.5-28.5T110-217q0-14 3-25.5t9-21.5l228-341q10-14 15-31t5-34v-170h220v170q0 17 5 34t15 31l227 341q7 9 10 20.5t3 26.5q0 41-28 69t-69 28H209Zm115-177q28 0 56-12t80-47q69-45 103.5-62.5T633-443q4-1 5.5-4.5t-.5-7.5l-78-117q-15-21-22.5-46t-7.5-52v-110H430v110q0 26-7.5 50.5T401-573L274-382q-3 4-9 28 0 23 17 40t42 17Zm26-483q-13 0-21.5-8.5T320-810q0-13 8.5-21.5T350-840h260q13 0 21.5 8.5T640-810q0 13-8.5 21.5T610-780H350Z"], [1, "smart-app-banner-text-container"], [1, "smart-app-banner-title"], [1, "smart-app-banner-subtitle"], [1, "smart-app-banner-actions"], ["flow-button", "", "matButton", "", "target", "_blank", "rel", "noopener noreferrer", 1, "smart-app-banner-open-button", 3, "click", "variant", "size", "href"], ["flow-icon-button", "", "matIconButton", "", "type", "button", 1, "smart-app-banner-dismiss-button", 3, "click", "variant", "aria-label"], [1, "flow-icon-m"]],
            template: function(a, b) {
                a & 1 && _.E(0, Tsb, 15, 7, "div", 0);
                a & 2 && _.F(b.isVisible() ? 0 : -1)
            },
            dependencies: [_.EH, _.MI, _.BH, _.jI, _.DH],
            styles: ['[_ngcontent-%COMP%]:root{--%NS%glass-raised-box-shadow:0 16px 32px -8px rgba(0,0,0,.4);--%NS%flow-panel-border-radius:20px}.blurred-backdrop[_ngcontent-%COMP%]{background-color:rgba(0,0,0,.32);backdrop-filter:blur(8px);-webkit-backdrop-filter:blur(8px)}mat-icon[_ngcontent-%COMP%]{font-variation-settings:"FILL" var(--%NS%flow-icon-fill,0),"wght" var(--%NS%flow-icon-wght,400)}mat-icon.flow-icon-s[_ngcontent-%COMP%]{font-size:1rem;height:1rem;width:1rem;line-height:1rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-m[_ngcontent-%COMP%]{font-size:1.125rem;height:1.125rem;width:1.125rem;line-height:1.125rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-l[_ngcontent-%COMP%]{font-size:1.25rem;height:1.25rem;width:1.25rem;line-height:1.25rem;--%NS%flow-icon-wght:400}mat-icon.flow-icon-xl[_ngcontent-%COMP%]{font-size:1.5rem;height:1.5rem;width:1.5rem;line-height:1.5rem;--%NS%flow-icon-wght:300}mat-icon.fill[_ngcontent-%COMP%]{--%NS%flow-icon-fill:1}@-webkit-keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@keyframes _ngcontent-%COMP%_shimmer{0%{-webkit-transform:translateX(-100%);transform:translateX(-100%)}to{-webkit-transform:translateX(100%);transform:translateX(100%)}}@-webkit-keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@keyframes _ngcontent-%COMP%_backdrop-unblur{0%{-webkit-backdrop-filter:blur(80px);backdrop-filter:blur(80px)}to{-webkit-backdrop-filter:blur(0);backdrop-filter:blur(0)}}@-webkit-keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}@keyframes _ngcontent-%COMP%_image-fade-in{0%{opacity:0}to{opacity:1}}[_nghost-%COMP%]{--%NS%smart-app-banner-background:#141516;--%NS%smart-app-banner-accent:#1a73e8;--%NS%smart-app-banner-accent-hover:#1b6ef3;display:block;width:100%}.smart-app-banner-container[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.75rem;height:3.5rem;padding:.625rem 1rem;background-color:var(--%NS%smart-app-banner-background);box-shadow:0 1px 0 var(--%NS%color-white-15);-moz-box-sizing:border-box;box-sizing:border-box;width:100%}.smart-app-banner-app-icon[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;-webkit-box-pack:center;-webkit-justify-content:center;-moz-box-pack:center;-ms-flex-pack:center;justify-content:center;width:2.25rem;height:2.25rem;min-width:2.25rem;border-radius:8px;background-color:var(--%NS%color-white);-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}.smart-app-banner-icon-symbol[_ngcontent-%COMP%]{width:1.5rem;height:1.5rem;display:block}.smart-app-banner-text-container[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-orient:vertical;-webkit-box-direction:normal;-webkit-flex-direction:column;-moz-box-orient:vertical;-moz-box-direction:normal;-ms-flex-direction:column;flex-direction:column;-webkit-box-flex:1;-webkit-flex:1;-moz-box-flex:1;-ms-flex:1;flex:1;min-width:0;gap:.125rem}.smart-app-banner-title[_ngcontent-%COMP%]{color:var(--%NS%color-white);font:var(--%NS%mat-sys-label-large);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.smart-app-banner-subtitle[_ngcontent-%COMP%]{color:var(--%NS%color-white-50);font:var(--%NS%mat-sys-body-small);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.smart-app-banner-actions[_ngcontent-%COMP%]{display:-webkit-box;display:-webkit-flex;display:-moz-box;display:-ms-flexbox;display:flex;-webkit-box-align:center;-webkit-align-items:center;-moz-box-align:center;-ms-flex-align:center;align-items:center;gap:.75rem;-webkit-flex-shrink:0;-ms-flex-negative:0;flex-shrink:0}a.smart-app-banner-open-button[_ngcontent-%COMP%]{--%NS%mat-button-filled-container-color:var(--%NS%smart-app-banner-accent);--%NS%mat-button-filled-container-shape:1rem;--%NS%mat-button-filled-label-text-color:var(--%NS%color-white);--%NS%mat-button-filled-hover-state-layer-opacity:0;--%NS%mat-button-filled-label-text-font:var(--%NS%mat-sys-label-large-font);--%NS%mat-button-filled-label-text-size:var(--%NS%mat-sys-label-large-size);--%NS%mat-button-filled-label-text-weight:var(--%NS%mat-sys-label-large-weight)}a.smart-app-banner-open-button[_ngcontent-%COMP%]:hover{background-color:var(--%NS%smart-app-banner-accent-hover)}button.smart-app-banner-dismiss-button[_ngcontent-%COMP%]{--%NS%mat-icon-button-icon-color:var(--%NS%smart-app-banner-accent);--%NS%mat-icon-button-state-layer-color:var(--%NS%smart-app-banner-accent)}']
        });
        _.Hv = class {
        }
        ;
        _.Hv.va = function(a) {
            return new (a || _.Hv)
        }
        ;
        _.Hv.Ma = _.z({
            type: _.Hv,
            Fa: [["flow-app-shell"]],
            Ka: 3,
            La: 0,
            template: function(a) {
                a & 1 && _.L(0, "flow-smart-app-banner")(1, "flow-app-header")(2, "router-outlet")
            },
            dependencies: [_.IZ, JZ, _.sD],
            styles: ["[_nghost-%COMP%]{display:contents}"]
        });
        _.Pm();
    } catch (e) {
        _._DumpException(e)
    }
}
).call(this, this.default_AiSandboxAngularFrontend);
// Google Inc.
