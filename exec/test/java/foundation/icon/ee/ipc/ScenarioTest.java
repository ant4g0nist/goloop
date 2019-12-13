package foundation.icon.ee.ipc;

import avm.Address;
import avm.Blockchain;
import foundation.icon.ee.test.Contract;
import foundation.icon.ee.test.GoldenTest;
import foundation.icon.ee.tooling.abi.External;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;

public class ScenarioTest extends GoldenTest {
    public static class Score {
        // short addr
        // short codeLen
        // byte[] code
        public static final byte CALL = 0;

        public static final byte REVERT = 1;

        // short len
        // byte[] string
        public static final byte SET_SVAR = 2;

        // short len
        // byte[] string
        public static final byte ADD_TO_SVAR = 3;

        // short len
        // byte[] string
        public static final byte EXPECT_SVAR = 4;

        private static String sVar;

        @External
        public static void run(byte[] code) {
            var ba = Blockchain.getAddress().toByteArray();
            int addr = (ba[1] << 8) & 0xff | (ba[2] & 0xff);
            Blockchain.println("Enter addr=" + addr);
            try {
                doRunImpl(code);
                Blockchain.println("Exit by Return addr=" + addr);
            } catch(Throwable t) {
                Blockchain.println("Exit by Throwable addr=" + addr + " t=" + t);
            }
        }

        private static void doRunImpl(byte[] code) {
            int offset = 0;
            while (offset < code.length) {
                int insn = code[offset++] & 0xff;
                if (insn == CALL) {
                    try {
                        var ba = new byte[21];
                        ba[0] = 1;
                        ba[1] = code[offset++];
                        ba[2] = code[offset++];
                        var addr = new Address(ba);
                        int ccodeLen = (code[offset++] << 8) & 0xff | (code[offset++] & 0xff);
                        var ccode = Arrays.copyOfRange(code, offset, offset + ccodeLen);
                        offset += ccodeLen;
                        Blockchain.call(addr, "run", new Object[]{ccode}, BigInteger.ZERO);
                    } catch (Exception e) {
                        Blockchain.println("Exception e=" + e);
                    }
                } else if (insn == REVERT){
                    var ba = Blockchain.getAddress().toByteArray();
                    int addr = (ba[1] << 8) & 0xff | (ba[2] & 0xff);
                    Blockchain.println("Exit by Revert addr=" + addr);
                    Blockchain.revert();
                } else if (insn == SET_SVAR) {
                    int len = (code[offset++] << 8) & 0xff | (code[offset++] & 0xff);
                    var s = new String(code, offset, len);
                    offset += len;
                    sVar = s;
                    Blockchain.println("Set sVar=" + sVar);
                } else if (insn == ADD_TO_SVAR) {
                    int len = (code[offset++] << 8) & 0xff | (code[offset++] & 0xff);
                    var s = new String(code, offset, len);
                    offset += len;
                    var before = sVar;
                    sVar += s;
                    Blockchain.println("AddTo sVar=" + before + " s=" + s + " => sVar=" + sVar);
                } else if (insn == EXPECT_SVAR) {
                    int len = (code[offset++] << 8) & 0xff | (code[offset++] & 0xff);
                    var s = new String(code, offset, len);
                    offset += len;
                    if (s.equals(sVar)) {
                        Blockchain.println("Expect [OK] expected sVar=" + sVar);
                    } else {
                        Blockchain.println("Expect [ERROR] expected sVar=" + s + " observed sVar=" + sVar);
                    }
                }
            }
        }
    }

    public static class Compiler {
        private static final int MAX_CODE = 8 * 1024;
        private ByteBuffer bb = ByteBuffer.allocate(MAX_CODE);
        private ArrayList<Integer> lengthOffsets = new ArrayList<>();

        public byte[] compile() {
            return Arrays.copyOf(bb.array(), bb.position());
        }

        public Compiler call(Contract c) {
            return call(c.getAddress());
        }

        public Compiler call(Address addr) {
            bb.put(Score.CALL);
            var ba = addr.toByteArray();
            bb.put(ba[1]);
            bb.put(ba[2]);
            lengthOffsets.add(bb.position());
            bb.putShort((short) 0);
            return this;
        }

        private void endCall() {
            int offset = lengthOffsets.remove(lengthOffsets.size() - 1);
            int len = bb.position() - offset - 2;
            bb.putShort(offset, (short) (len & 0xffff));
        }

        public Compiler ret() {
            endCall();
            return this;
        }

        public Compiler revert() {
            bb.put(Score.REVERT);
            endCall();
            return this;
        }

        public Compiler setSVar(String s) {
            bb.put(Score.SET_SVAR);
            bb.putShort((short) (s.length() & 0xffff));
            bb.put(s.getBytes(StandardCharsets.UTF_8));
            return this;
        }

        public Compiler addToSVar(String s) {
            bb.put(Score.ADD_TO_SVAR);
            bb.putShort((short) (s.length() & 0xffff));
            bb.put(s.getBytes(StandardCharsets.UTF_8));
            return this;
        }

        public Compiler expectSVar(String s) {
            bb.put(Score.EXPECT_SVAR);
            bb.putShort((short) (s.length() & 0xffff));
            bb.put(s.getBytes(StandardCharsets.UTF_8));
            return this;
        }
    }

    @Test
    public void testBasic() {
        var c1 = sm.deploy(Score.class);
        var c2 = sm.deploy(Score.class);
        var c3 = sm.deploy(Score.class);
        var code = new Compiler()
                .call(c2)
                .ret()
                .call(c3)
                    .call(c2)
                    .ret()
                .revert()
                .compile();
        c1.invoke("run", (Object)code);
    }

    @Test
    public void testIndirectRecursion() {
        var c1 = sm.deploy(Score.class);
        var c2 = sm.deploy(Score.class);
        var c3 = sm.deploy(Score.class);
        c1.invoke("run", (Object)new Compiler()
                .call(c2)
                    .setSVar("")
                .ret()
                .call(c2)
                    .addToSVar("a")
                .revert()
                .call(c2)
                    .addToSVar("b")
                .ret()
                .call(c3)
                    .call(c2)
                        .addToSVar("c")
                        .expectSVar("bc")
                    .revert()
                .revert()
                .call(c3)
                    .call(c2)
                        .addToSVar("d")
                    .ret()
                .revert()
                .call(c3)
                    .call(c2)
                        .addToSVar("e")
                    .revert()
                .ret()
                .call(c3)
                    .call(c2)
                        .addToSVar("f")
                    .ret()
                .ret()
                .call(c2)
                    .expectSVar("bf")
                .ret()
                .compile()
        );
    }

    @Test
    public void testDirectRecursion() {
        var c1 = sm.deploy(Score.class);
        c1.invoke("run", (Object)new Compiler()
                .setSVar("")
                .call(c1)
                    .addToSVar("a")
                .revert()
                .expectSVar("")
                .call(c1)
                    .addToSVar("b")
                .ret()
                .expectSVar("b")
                .call(c1)
                    .call(c1)
                        .addToSVar("c")
                        .expectSVar("bc")
                    .revert()
                    .expectSVar("b")
                .revert()
                .call(c1)
                    .call(c1)
                        .addToSVar("d")
                        .expectSVar("bd")
                    .ret()
                    .expectSVar("bd")
                .revert()
                .expectSVar("b")
                .call(c1)
                    .call(c1)
                        .addToSVar("e")
                        .expectSVar("be")
                    .revert()
                    .expectSVar("b")
                .ret()
                .expectSVar("b")
                .call(c1)
                    .call(c1)
                        .addToSVar("f")
                        .expectSVar("bf")
                    .ret()
                    .expectSVar("bf")
                .ret()
                .expectSVar("bf")
                .compile()
        );
    }
}
